package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenIssuer struct {
	secretKey []byte
	expiry    time.Duration
}

func NewTokenIssuer(secret string, expiry time.Duration) *TokenIssuer {
	return &TokenIssuer{
		secretKey: []byte(secret),
		expiry:    expiry,
	}
}

func (t *TokenIssuer) Create(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"userId": userID,
			"exp":    time.Now().Add(t.expiry).Unix(),
		})

	tokenString, err := token.SignedString(t.secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
