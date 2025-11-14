package auth

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var secretKey = []byte("secret-key")

type Claims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

func verifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
