// Command gamepulse-server boots the GamePulse API and background workers.
//
// In production it expects DATABASE_URL plus credentials for the sports,
// news, AI and SMS providers. When any provider credentials are missing the
// service falls back to the bundled mock implementations so the full flow
// remains exercisable in development and CI.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gamepulse/backend/internal/api"
	"github.com/gamepulse/backend/internal/config"
	"github.com/gamepulse/backend/internal/db"
	"github.com/gamepulse/backend/internal/logger"
	"github.com/gamepulse/backend/internal/providers/ai"
	"github.com/gamepulse/backend/internal/providers/news"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/providers/sports"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/gamepulse/backend/internal/services"
	"github.com/gamepulse/backend/internal/workers"
)

func main() {
	log := logger.New()
	cfg := config.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		log.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	log.Info("migrations applied")

	r := repo.New(pool)

	smsSender := chooseSMS(cfg, log)
	sportsProv := chooseSports(cfg, log)
	newsProv := chooseNews(cfg, log)
	aiSummarizer := chooseAI(cfg, log)

	subSvc := services.NewSubscriptionService(r, smsSender, log)
	subSvc.InitialNewsDelay = cfg.InitialNewsDelay
	dispatcher := services.NewDispatcher(r, smsSender, log)

	server := api.New(r, subSvc, smsSender, log)

	if cfg.EnableWorkers {
		newsAgg := &workers.NewsAggregator{
			Repo: r, News: newsProv, Log: log,
			Interval: cfg.NewsAggregatorInterval,
		}
		go (&workers.LiveTracker{
			Repo: r, Sports: sportsProv, Dispatcher: dispatcher, Log: log,
			Interval:              cfg.LiveTrackerInterval,
			FinishedGameRetention: cfg.FinishedGameRetention,
		}).Run(ctx)
		go newsAgg.Run(ctx)
		go (&workers.DigestBuilder{
			Repo: r, SMS: smsSender, AI: aiSummarizer, Log: log,
			News:             newsAgg,
			Interval:         cfg.DigestInterval,
			ArticleRetention: cfg.NewsArticleRetention,
			Now:              func() time.Time { return time.Now().UTC() },
		}).Run(ctx)
	}

	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr)
		if err := server.App.Listen(cfg.HTTPAddr); err != nil {
			log.Error("http listen failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdown, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()
	_ = server.App.ShutdownWithContext(shutdown)
}

// --- Provider selection --------------------------------------------------

func chooseSMS(cfg *config.Config, log *slog.Logger) sms.Sender {
	if cfg.TwilioAccountSID != "" && cfg.TwilioAuthToken != "" && cfg.TwilioFromNumber != "" {
		log.Info("sms provider", "type", "twilio")
		return sms.NewTwilio(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFromNumber)
	}
	log.Info("sms provider", "type", "log (no twilio creds)")
	return sms.NewLogSender(log)
}

func chooseSports(cfg *config.Config, log *slog.Logger) sports.Provider {
	// Default to ESPN's free public scoreboard. Set SPORTS_PROVIDER=mock
	// to force the deterministic simulator (useful for demos and tests).
	switch cfg.SportsProvider {
	case "mock":
		log.Info("sports provider", "type", "mock")
		return sports.NewMockProvider()
	default:
		log.Info("sports provider", "type", "espn")
		return sports.NewESPN()
	}
}

func chooseNews(cfg *config.Config, log *slog.Logger) news.Provider {
	if cfg.NewsAPIKey != "" {
		log.Info("news provider", "type", "newsapi.org")
		return news.NewNewsAPI(cfg.NewsAPIKey)
	}
	log.Info("news provider", "type", "mock")
	return news.NewMockProvider()
}

func chooseAI(cfg *config.Config, log *slog.Logger) ai.Summarizer {
	if cfg.AIAPIKey != "" {
		log.Info("ai provider", "type", "openai")
		return ai.NewOpenAI(cfg.AIAPIKey)
	}
	log.Info("ai provider", "type", "heuristic")
	return ai.NewHeuristicSummarizer()
}
