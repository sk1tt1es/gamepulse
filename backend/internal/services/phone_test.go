package services

import (
	"errors"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"+14155550123", "+14155550123", false},
		{"  +1 (415) 555-0123 ", "+14155550123", false},
		{"+1.415.555.0123", "+14155550123", false},
		{"4155550123", "", true},                  // missing +
		{"+0445550123", "", true},                 // leading 0 country code
		{"+1234567890123456", "", true},           // too long
		{"", "", true},
		{"+", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizePhone(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizePhone(%q) expected error, got %q", tc.in, got)
			} else if !errors.Is(err, ErrInvalidPhone) {
				t.Errorf("NormalizePhone(%q) wrong error: %v", tc.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizePhone(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
