package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Missing or invalid authorization header")
			return
		}
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		if err := verifyToken(tokenString); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
