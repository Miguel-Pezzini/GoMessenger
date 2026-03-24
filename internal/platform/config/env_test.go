package config

import "testing"

func TestStringReturnsFallbackWhenUnset(t *testing.T) {
	t.Setenv("CONFIG_TEST_UNSET", "")

	if got := String("CONFIG_TEST_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestStringReturnsEnvironmentValue(t *testing.T) {
	t.Setenv("CONFIG_TEST_SET", "value")

	if got := String("CONFIG_TEST_SET", "fallback"); got != "value" {
		t.Fatalf("expected env value, got %q", got)
	}
}
