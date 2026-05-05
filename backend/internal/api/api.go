// Package api wires the HTTP layer. We use Fiber as the web framework
// because it's fast, has a small footprint, and ships with everything we
// need (logger, recover, CORS) out of the box.
package api

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/gamepulse/backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
)

type Server struct {
	App *fiber.App
	Sub *services.SubscriptionService
	Rep *repo.Repo
	SMS sms.Sender
	Log *slog.Logger
}

func New(
	rep *repo.Repo,
	sub *services.SubscriptionService,
	smsSender sms.Sender,
	log *slog.Logger,
) *Server {
	app := fiber.New(fiber.Config{
		AppName:               "GamePulse",
		DisableStartupMessage: true,
		ErrorHandler:          errorHandler,
	})
	app.Use(recover.New())
	app.Use(fiberlogger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Accept",
	}))

	s := &Server{App: app, Sub: sub, Rep: rep, SMS: smsSender, Log: log}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.App.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	v1 := s.App.Group("/api/v1")
	v1.Get("/teams", s.listTeams)
	v1.Post("/subscriptions", s.createSubscription)
	v1.Delete("/subscriptions/:id", s.deleteSubscription)

	// Debug endpoint exposes the in-memory log of SMS messages when running
	// without Twilio credentials. Useful for the demo & E2E tests.
	v1.Get("/debug/sms", s.debugSMS)

	// Twilio inbound webhook for STOP / HELP / START. Mounted at the
	// top level (not under /api/v1) because the URL is what you paste
	// into the Twilio console; keeping it short reduces transcription
	// errors when copy/pasting and matches Twilio's own conventions.
	s.App.Post("/sms/inbound", s.inboundSMS)
}

// errorHandler converts known sentinel errors to appropriate status codes
// and ensures every error body is JSON.
func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if fe, ok := err.(*fiber.Error); ok {
		code = fe.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}

// --- Handlers ------------------------------------------------------------

type teamGroup struct {
	League string         `json:"league"`
	Teams  []models.Team  `json:"teams"`
}

func (s *Server) listTeams(c *fiber.Ctx) error {
	teams, err := s.Rep.ListTeams(c.Context())
	if err != nil {
		return err
	}

	// Group by league. We preserve a stable league order matching the spec.
	order := []models.League{models.LeagueNBA, models.LeagueNFL, models.LeagueMLB, models.LeagueNHL}
	byLeague := map[models.League][]models.Team{}
	for _, t := range teams {
		byLeague[t.League] = append(byLeague[t.League], t)
	}
	out := make([]teamGroup, 0, len(order))
	for _, l := range order {
		out = append(out, teamGroup{League: string(l), Teams: byLeague[l]})
	}
	return c.JSON(fiber.Map{"leagues": out})
}

type createSubscriptionReq struct {
	PhoneNumber string `json:"phone_number"`
	TeamID      string `json:"team_id"`
	UpdateType  string `json:"update_type"`
	Frequency   string `json:"frequency"`
}

func (s *Server) createSubscription(c *fiber.Ctx) error {
	var req createSubscriptionReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	teamID, err := uuid.Parse(strings.TrimSpace(req.TeamID))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "team_id must be a valid uuid")
	}
	in := services.SubscriptionInput{
		PhoneNumber: req.PhoneNumber,
		TeamID:      teamID,
		UpdateType:  models.UpdateType(strings.ToLower(req.UpdateType)),
		Frequency:   models.Frequency(strings.ToLower(req.Frequency)),
	}
	sub, team, err := s.Sub.Create(c.Context(), in)
	switch {
	case errors.Is(err, services.ErrInvalidPhone):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrDuplicateSubscription):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, repo.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, "team not found")
	case err != nil:
		// Validation failures from SubscriptionInput.Validate.
		if strings.HasPrefix(err.Error(), "update_type must") || strings.HasPrefix(err.Error(), "frequency must") {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"subscription": sub,
		"team":         team,
		"message":      "Confirmation SMS sent.",
	})
}

func (s *Server) deleteSubscription(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id must be a valid uuid")
	}
	if err := s.Rep.DeleteSubscription(c.Context(), id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "subscription not found")
		}
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) debugSMS(c *fiber.Ctx) error {
	ls, ok := s.SMS.(*sms.LogSender)
	if !ok {
		return c.JSON(fiber.Map{"messages": []any{}, "note": "live SMS provider is not LogSender"})
	}
	return c.JSON(fiber.Map{"messages": ls.Sent()})
}
