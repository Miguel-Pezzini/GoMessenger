package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Handler struct {
	proxy *httputil.ReverseProxy
}

func NewHandler(target string) (*Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if userID, ok := req.Context().Value(UserIDKey).(string); ok && userID != "" {
			req.Header.Set("X-User-ID", userID)
		}
		if role, ok := req.Context().Value(UserRoleKey).(string); ok && role != "" {
			req.Header.Set("X-User-Role", role)
		}
	}

	return &Handler{proxy: proxy}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.proxy.ServeHTTP(w, r)
}
