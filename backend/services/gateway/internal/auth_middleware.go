package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const UserIDKey contextKey = "userId"
const UserRoleKey contextKey = "role"

type JWTMiddleware struct {
	verifier *Verifier
}

func New(secrets ...string) *JWTMiddleware {
	return &JWTMiddleware{verifier: NewVerifier(secrets...)}
}

func (m *JWTMiddleware) Wrap(next http.Handler) http.Handler {
	return m.wrap(next, "")
}

func (m *JWTMiddleware) WrapAdmin(next http.Handler) http.Handler {
	return m.wrap(next, "ADMIN")
}

func (m *JWTMiddleware) wrap(next http.Handler, requiredRole string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractToken(r)
		if tokenString == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing token")
			return
		}

		claims, err := m.verifier.Verify(tokenString)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		if requiredRole != "" && claims.Role != requiredRole {
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserRoleKey, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractToken(r *http.Request) string {
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
			return strings.TrimSpace(token)
		}
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	return ""
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
