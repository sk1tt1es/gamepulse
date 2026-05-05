package ai

import (
	"context"
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	short := "Hello world"
	if got := Truncate(short); got != short {
		t.Errorf("Truncate(short) = %q, want %q", got, short)
	}

	long := strings.Repeat("a", MaxSummaryChars+50)
	got := Truncate(long)
	if len(got) > MaxSummaryChars+1 { // +1 for ellipsis rune (3 bytes utf8)
		// Allow ellipsis byte expansion: maxChars + utf8 ellipsis (3 bytes)
		if len(got) > MaxSummaryChars+3 {
			t.Errorf("Truncate result too long: %d > %d", len(got), MaxSummaryChars+3)
		}
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("Expected ellipsis suffix, got %q", got)
	}
}

func TestHeuristicSummarizer(t *testing.T) {
	s := NewHeuristicSummarizer()
	out, err := s.Summarize(context.Background(), "Title",
		"First sentence here. Second one extends it. A long third sentence that should not be included.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "First sentence") {
		t.Errorf("expected first sentence, got %q", out)
	}
	if strings.Contains(out, "third sentence") {
		t.Errorf("did not expect third sentence: %q", out)
	}

	// Empty content falls back to title.
	out, err = s.Summarize(context.Background(), "Just a title", "")
	if err != nil {
		t.Fatal(err)
	}
	if out != "Just a title" {
		t.Errorf("expected title fallback, got %q", out)
	}
}

func TestSplitSentences(t *testing.T) {
	sents := splitSentences("One. Two? Three!")
	if len(sents) != 3 {
		t.Fatalf("expected 3 sentences, got %d (%v)", len(sents), sents)
	}
}
