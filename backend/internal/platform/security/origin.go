package security

import (
	"net/http"
	"strings"
)

type OriginValidator struct {
	allowed map[string]struct{}
}

func NewOriginValidator(origins []string) OriginValidator {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		allowed[origin] = struct{}{}
	}

	return OriginValidator{allowed: allowed}
}

func (v OriginValidator) IsAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if len(v.allowed) == 0 {
		return false
	}
	_, ok := v.allowed[origin]
	return ok
}
