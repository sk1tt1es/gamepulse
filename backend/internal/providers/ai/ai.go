// Package ai abstracts the LLM used to summarize news articles. The
// production backend calls an OpenAI-compatible Chat Completions endpoint;
// the fallback uses a deterministic local heuristic so the news pipeline
// remains testable without an API key.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// MaxSummaryChars is the cap we enforce regardless of backend, mirroring
// the SMS-friendly constraint stated in the spec.
const MaxSummaryChars = 280

type Summarizer interface {
	Summarize(ctx context.Context, title, content string) (string, error)
}

// --- OpenAI-compatible backend -------------------------------------------

type OpenAI struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

func NewOpenAI(key string) *OpenAI {
	return &OpenAI{
		APIKey:  key,
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

type chatReq struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

func (o *OpenAI) Summarize(ctx context.Context, title, content string) (string, error) {
	prompt := fmt.Sprintf(
		"Summarize the following sports news article in 1-2 sentences for SMS. "+
			"Keep the tone factual and under %d characters. Do not add hashtags or emojis.\n\n"+
			"Title: %s\n\nArticle:\n%s",
		MaxSummaryChars, title, content,
	)
	body, _ := json.Marshal(chatReq{
		Model: o.Model,
		Messages: []message{
			{Role: "system", Content: "You write concise, factual sports news summaries for SMS."},
			{Role: "user", Content: prompt},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai non-2xx: %d", resp.StatusCode)
	}
	var out chatResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}
	return Truncate(out.Choices[0].Message.Content), nil
}

// --- Heuristic backend ---------------------------------------------------

// HeuristicSummarizer produces a "good enough" summary by taking the first
// 1-2 sentences and trimming to MaxSummaryChars. It's used when no AI key
// is configured. The behaviour is deterministic, which is helpful for tests.
type HeuristicSummarizer struct{}

func NewHeuristicSummarizer() *HeuristicSummarizer { return &HeuristicSummarizer{} }

func (HeuristicSummarizer) Summarize(_ context.Context, title, content string) (string, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return Truncate(title), nil
	}
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return Truncate(text), nil
	}
	summary := sentences[0]
	if len(sentences) > 1 && len(summary) < 160 {
		summary = strings.TrimSpace(summary + " " + sentences[1])
	}
	return Truncate(summary), nil
}

func splitSentences(s string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range s {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			if str := strings.TrimSpace(cur.String()); str != "" {
				out = append(out, str)
			}
			cur.Reset()
		}
	}
	if str := strings.TrimSpace(cur.String()); str != "" {
		out = append(out, str)
	}
	return out
}

// Truncate enforces MaxSummaryChars on UTF-8 input, breaking on the last
// word boundary and appending an ellipsis if it had to cut.
func Truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= MaxSummaryChars {
		return s
	}
	cut := s[:MaxSummaryChars]
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}
