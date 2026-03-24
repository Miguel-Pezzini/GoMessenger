package websocketproxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/middleware"
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
		if userID, ok := req.Context().Value(middleware.UserIDKey).(string); ok && userID != "" {
			req.Header.Set("X-User-ID", userID)
		}
	}

	return &Handler{proxy: proxy}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.proxy.ServeHTTP(w, r)
}
