package gateway

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func makeToken(t *testing.T, userID, role, secret string, expiry time.Duration) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userID,
		"role":   role,
		"exp":    time.Now().Add(expiry).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return signed
}

func TestVerifyValidTokenReturnsClaims(t *testing.T) {
	v := NewVerifier("my-secret")
	tokenString := makeToken(t, "user-123", "ADMIN", "my-secret", time.Hour)

	claims, err := v.Verify(tokenString)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Fatalf("expected userID=user-123, got %s", claims.UserID)
	}
	if claims.Role != "ADMIN" {
		t.Fatalf("expected role=ADMIN, got %s", claims.Role)
	}
}

func TestVerifyWrongSecretReturnsError(t *testing.T) {
	v := NewVerifier("correct-secret")
	tokenString := makeToken(t, "user-1", "USER", "wrong-secret", time.Hour)

	_, err := v.Verify(tokenString)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestVerifyExpiredTokenReturnsError(t *testing.T) {
	v := NewVerifier("my-secret")
	tokenString := makeToken(t, "user-1", "USER", "my-secret", -time.Second)

	_, err := v.Verify(tokenString)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestVerifyMalformedTokenReturnsError(t *testing.T) {
	v := NewVerifier("my-secret")
	_, err := v.Verify("this.is.not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

func TestVerifyEmptyTokenReturnsError(t *testing.T) {
	v := NewVerifier("my-secret")
	_, err := v.Verify("")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestVerifyAcceptsPreviousSecretDuringRotation(t *testing.T) {
	v := NewVerifier("current-secret", "previous-secret")
	tokenString := makeToken(t, "user-1", "USER", "previous-secret", time.Hour)

	claims, err := v.Verify(tokenString)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("expected user-1, got %s", claims.UserID)
	}
}
