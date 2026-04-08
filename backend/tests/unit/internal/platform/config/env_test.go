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

func TestMustStringReturnsEnvironmentValue(t *testing.T) {
	t.Setenv("CONFIG_TEST_REQUIRED", "value")

	if got := MustString("CONFIG_TEST_REQUIRED"); got != "value" {
		t.Fatalf("expected env value, got %q", got)
	}
}

func TestMustStringPanicsWhenUnset(t *testing.T) {
	t.Setenv("CONFIG_TEST_REQUIRED_MISSING", "")

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing environment variable")
		}
	}()

	_ = MustString("CONFIG_TEST_REQUIRED_MISSING")
}
