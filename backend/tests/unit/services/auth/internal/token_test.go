package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenIssuerCreateReturnsSignedToken(t *testing.T) {
	issuer := NewTokenIssuer("my-secret", time.Hour)
	token, err := issuer.Create("user-123", RoleUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestTokenIssuerTokenContainsUserID(t *testing.T) {
	issuer := NewTokenIssuer("my-secret", time.Hour)
	tokenString, err := issuer.Create("user-abc", RoleUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("my-secret"), nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		t.Fatal("invalid token claims")
	}
	if claims["userId"] != "user-abc" {
		t.Fatalf("expected userId=user-abc, got %v", claims["userId"])
	}
}

func TestTokenIssuerTokenExpiry(t *testing.T) {
	issuer := NewTokenIssuer("my-secret", time.Hour)
	tokenString, _ := issuer.Create("user-1", RoleUser)

	parsed, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("my-secret"), nil
	})

	claims := parsed.Claims.(jwt.MapClaims)
	exp := int64(claims["exp"].(float64))

	now := time.Now().Unix()
	if exp <= now {
		t.Fatal("expected token to expire in the future")
	}
	if exp > now+int64(2*time.Hour/time.Second) {
		t.Fatal("expected token to expire within 2 hours")
	}
}

func TestTokenIssuerTokenContainsRole(t *testing.T) {
	issuer := NewTokenIssuer("my-secret", time.Hour)
	tokenString, err := issuer.Create("user-abc", RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("my-secret"), nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims := parsed.Claims.(jwt.MapClaims)
	if claims["role"] != RoleAdmin {
		t.Fatalf("expected role=%s, got %v", RoleAdmin, claims["role"])
	}
}

func TestTokenIssuerDifferentSecretCannotVerify(t *testing.T) {
	issuer := NewTokenIssuer("secret-A", time.Hour)
	tokenString, _ := issuer.Create("user-1", RoleUser)

	_, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte("secret-B"), nil
	})
	if err == nil {
		t.Fatal("expected verification to fail with a different secret")
	}
}
