package services

import (
	"errors"
	"regexp"
	"strings"
)

// e164 enforces the spec-required phone number shape: a leading "+",
// country code starting with 1-9, and 1-14 additional digits (15 total).
var e164 = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// ErrInvalidPhone is returned when a phone number doesn't match E.164.
var ErrInvalidPhone = errors.New("phone number must be E.164 (e.g. +14155550123)")

// NormalizePhone trims whitespace, strips spaces and dashes commonly
// included by users, and validates the final shape against E.164.
func NormalizePhone(in string) (string, error) {
	p := strings.TrimSpace(in)
	p = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(p)
	if !e164.MatchString(p) {
		return "", ErrInvalidPhone
	}
	return p, nil
}
