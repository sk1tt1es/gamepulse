package sms

import (
	"context"
	"testing"
)

func TestLogSender_RecordsMessages(t *testing.T) {
	ls := NewLogSender(nil)
	if err := ls.Send(context.Background(), "+1", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := ls.Send(context.Background(), "+2", "world"); err != nil {
		t.Fatal(err)
	}
	got := ls.Sent()
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].To != "+1" || got[0].Body != "hello" {
		t.Errorf("unexpected first message: %+v", got[0])
	}
	if got[1].To != "+2" || got[1].Body != "world" {
		t.Errorf("unexpected second message: %+v", got[1])
	}
}
