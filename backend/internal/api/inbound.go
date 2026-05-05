package api

import (
	"errors"
	"strings"

	"github.com/gamepulse/backend/internal/repo"
	"github.com/gofiber/fiber/v2"
)

// Twilio Programmable Messaging POSTs inbound messages to a configured
// webhook URL with `application/x-www-form-urlencoded`. We only need two
// fields: `From` (the sender's E.164 phone number) and `Body` (the SMS
// text). The response we send back is TwiML XML — Twilio renders the
// `<Message>` element back to the sender automatically, so we don't need
// to call the REST API to reply.
//
// A2P 10DLC compliance requires two specific keywords:
//
//   - STOP / STOPALL / UNSUBSCRIBE / CANCEL / END / QUIT → unsubscribe
//   - HELP / INFO                                        → help text
//
// Carriers also auto-handle these on Twilio's side, but mirroring the
// behavior in our DB keeps subscription state consistent with what the
// subscriber actually has on their phone.

func (s *Server) inboundSMS(c *fiber.Ctx) error {
	from := strings.TrimSpace(c.FormValue("From"))
	body := strings.ToUpper(strings.TrimSpace(c.FormValue("Body")))

	c.Type("xml")

	// Empty / unparseable inbound — return blank TwiML so Twilio doesn't
	// retry. We log so unexpected payload shapes are visible.
	if from == "" {
		s.Log.Warn("inbound sms missing From", "body", body)
		return c.SendString(twimlEmpty())
	}

	// Strip leading word so phrases like "STOP please" still match.
	first := body
	if i := strings.IndexAny(body, " \t\n"); i > 0 {
		first = body[:i]
	}

	switch first {
	case "STOP", "STOPALL", "UNSUBSCRIBE", "CANCEL", "END", "QUIT":
		removed := s.handleStop(c, from)
		s.Log.Info("inbound stop", "from", from, "removed", removed)
		return c.SendString(twimlReply(
			"You've been unsubscribed from GamePulse. " +
				"You will not receive any more messages. " +
				"Reply START to re-subscribe.",
		))

	case "HELP", "INFO":
		return c.SendString(twimlReply(
			"GamePulse: live sports score and news updates. " +
				"Msg & data rates may apply. Reply STOP to cancel. " +
				"Support: support@gamepulse.example",
		))

	case "START", "UNSTOP", "YES":
		// Twilio re-enables delivery automatically on START. We simply
		// acknowledge — the user must visit the website to pick a team
		// (we deliberately don't auto-recreate prior subscriptions to
		// avoid sending unwanted messages).
		return c.SendString(twimlReply(
			"Welcome back to GamePulse! " +
				"Visit the website to choose a team and update preferences.",
		))
	}

	// Unknown keyword — short reply pointing at HELP.
	return c.SendString(twimlReply(
		"GamePulse received your message. Reply HELP for info or STOP to unsubscribe.",
	))
}

// handleStop deletes every subscription tied to the given phone number.
// Returns the number of subscriptions removed (0 when the phone is
// unknown). Never errors — STOP must always succeed from the user's
// perspective.
func (s *Server) handleStop(c *fiber.Ctx, phone string) int {
	user, err := s.Rep.FindUserByPhone(c.Context(), phone)
	if errors.Is(err, repo.ErrNotFound) {
		return 0
	}
	if err != nil {
		s.Log.Warn("stop user lookup failed", "err", err)
		return 0
	}
	n, err := s.Rep.DeleteSubscriptionsForUser(c.Context(), user.ID)
	if err != nil {
		s.Log.Warn("stop delete failed", "err", err)
	}
	return n
}

func twimlReply(body string) string {
	// Minimal TwiML. We use no `<Sms>` attributes so Twilio replies from
	// the same number that received the inbound message.
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Response><Message>` + escapeXML(body) + `</Message></Response>`
}

func twimlEmpty() string {
	return `<?xml version="1.0" encoding="UTF-8"?><Response/>`
}

func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}
