package gateway

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Verifier struct {
	secretKeys [][]byte
}

func NewVerifier(secrets ...string) *Verifier {
	keys := make([][]byte, 0, len(secrets))
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		keys = append(keys, []byte(secret))
	}

	return &Verifier{secretKeys: keys}
}

func (v *Verifier) Verify(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("invalid token")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}

		for _, secretKey := range v.secretKeys {
			if len(secretKey) == 0 {
				continue
			}
			return secretKey, nil
		}

		return nil, errors.New("missing jwt secret")
	})

	if err != nil {
		for _, secretKey := range v.secretKeys[1:] {
			token, retryErr := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, errors.New("unexpected signing method")
				}
				return secretKey, nil
			})
			if retryErr == nil {
				if claims, ok := token.Claims.(*Claims); ok && token.Valid {
					return claims, nil
				}
			}
		}
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
