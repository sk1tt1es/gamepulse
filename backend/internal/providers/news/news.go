// Package news abstracts news article retrieval. The default mock generates
// rotating "headlines" per team so the rest of the pipeline (AI summarizer
// + dispatcher) can be exercised without a NewsAPI key.
package news

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gamepulse/backend/internal/models"
)

type Article struct {
	Title       string
	Content     string
	Source      string
	URL         string
	PublishedAt time.Time
}

type Provider interface {
	// Fetch returns recent articles for the given team. Implementations
	// should bound results to a recent window to keep the workload small.
	Fetch(ctx context.Context, team models.Team) ([]Article, error)
}

// --- NewsAPI.org backed implementation -----------------------------------

type NewsAPI struct {
	APIKey string
	HTTP   *http.Client
}

func NewNewsAPI(key string) *NewsAPI {
	return &NewsAPI{APIKey: key, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

type newsAPIResponse struct {
	Articles []struct {
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Content     string    `json:"content"`
		URL         string    `json:"url"`
		PublishedAt time.Time `json:"publishedAt"`
		Source      struct {
			Name string `json:"name"`
		} `json:"source"`
	} `json:"articles"`
}

func (n *NewsAPI) Fetch(ctx context.Context, team models.Team) ([]Article, error) {
	q := url.Values{}
	q.Set("q", fmt.Sprintf("%q", team.Name))
	q.Set("language", "en")
	q.Set("sortBy", "publishedAt")
	q.Set("pageSize", "10")
	q.Set("apiKey", n.APIKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://newsapi.org/v2/everything?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := n.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("newsapi non-2xx: %d", resp.StatusCode)
	}
	var body newsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]Article, 0, len(body.Articles))
	for _, a := range body.Articles {
		content := a.Content
		if content == "" {
			content = a.Description
		}
		out = append(out, Article{
			Title: a.Title, Content: content, Source: a.Source.Name,
			URL: a.URL, PublishedAt: a.PublishedAt,
		})
	}
	return out, nil
}

// --- Mock implementation -------------------------------------------------

// MockProvider returns a deterministic, slowly-rotating set of headlines per
// team. The headline index advances on every call so each aggregator run
// surfaces "new" content for the dispatcher.
type MockProvider struct {
	mu      sync.Mutex
	counter map[string]int
}

func NewMockProvider() *MockProvider {
	return &MockProvider{counter: map[string]int{}}
}

var headlineTemplates = []struct {
	Title   string
	Content string
}{
	{
		"%s star player questionable for tonight's game",
		"Reports indicate the star player is dealing with a minor injury and is listed as day-to-day. The team is monitoring the situation closely as they prepare for tonight's matchup.",
	},
	{
		"%s extend winning streak with dominant performance",
		"Behind a balanced offensive attack and stout defense, the team extended their winning streak to five games. The bench provided crucial minutes to seal the result.",
	},
	{
		"Coach of %s addresses recent slump",
		"The head coach met with reporters and outlined adjustments to practice and rotations following a string of disappointing results. He expressed confidence in the locker room.",
	},
	{
		"%s announce roster move ahead of trade deadline",
		"The front office finalized a depth move to bolster the roster ahead of the trade deadline. The acquired player is expected to contribute immediately off the bench.",
	},
	{
		"%s celebrate franchise milestone",
		"A long-tenured player crossed a major statistical milestone in last night's game. Teammates and fans recognized the achievement during a postgame ceremony.",
	},
}

func (m *MockProvider) Fetch(_ context.Context, team models.Team) ([]Article, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.counter[team.ID.String()]
	m.counter[team.ID.String()] = idx + 1
	tpl := headlineTemplates[idx%len(headlineTemplates)]
	pub := time.Now().UTC()
	return []Article{
		{
			Title:       fmt.Sprintf(tpl.Title, team.Name),
			Content:     tpl.Content,
			Source:      "GamePulse Wire",
			URL:         fmt.Sprintf("https://example.com/news/%s/%d", team.ExternalID, idx),
			PublishedAt: pub,
		},
	}, nil
}
