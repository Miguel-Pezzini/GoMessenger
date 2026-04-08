package auth

import (
	"testing"
	"time"
)

const testJWTExpiry = time.Hour

func TestParseJWTExpiryUsesDefaultWhenUnset(t *testing.T) {
	if got := parseJWTExpiry(""); got != 24*time.Hour {
		t.Fatalf("expected default expiry 24h, got %v", got)
	}
}

func TestParseJWTExpiryParsesDuration(t *testing.T) {
	if got := parseJWTExpiry("15m"); got != 15*time.Minute {
		t.Fatalf("expected 15m, got %v", got)
	}
}
