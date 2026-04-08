package gateway

import (
	"net/http"
	"strings"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	"log"
)

type Config struct {
	Address             string
	AuthURL             string
	FriendsURL          string
	WebsocketURL        string
	ChatURL             string
	PresenceURL         string
	LoggingURL          string
	AllowedOrigin       string
	JWTSecret           string
	JWTPreviousSecrets  []string
	FriendEventsChannel string
}

func LoadConfig() Config {
	return Config{
		Address:             config.MustString("GATEWAY_ADDR"),
		AuthURL:             config.MustString("AUTH_UPSTREAM_URL"),
		FriendsURL:          config.MustString("FRIENDS_UPSTREAM_URL"),
		WebsocketURL:        config.MustString("WEBSOCKET_UPSTREAM_URL"),
		ChatURL:             config.MustString("CHAT_UPSTREAM_URL"),
		PresenceURL:         config.MustString("PRESENCE_UPSTREAM_URL"),
		LoggingURL:          config.MustString("LOGGING_UPSTREAM_URL"),
		AllowedOrigin:       config.MustString("GATEWAY_ALLOWED_ORIGIN"),
		JWTSecret:           config.MustString("JWT_SECRET"),
		JWTPreviousSecrets:  parseSecretList(config.String("JWT_SECRET_PREVIOUS", "")),
		FriendEventsChannel: config.MustString("REDIS_CHANNEL_FRIEND_EVENTS"),
	}
}

func Run() error {
	cfg := LoadConfig()
	router, err := NewRouter(cfg)
	if err != nil {
		return err
	}

	log.Printf("gateway listening on %s", cfg.Address)
	return http.ListenAndServe(cfg.Address, router)
}

func NewRouter(cfg Config) (http.Handler, error) {
	mux := http.NewServeMux()

	jwtMiddleware := New(append([]string{cfg.JWTSecret}, cfg.JWTPreviousSecrets...)...)
	newUpstreamHandler := func(target string) (http.Handler, error) {
		return NewHandler(target)
	}

	authUpstream, err := newUpstreamHandler(cfg.AuthURL)
	if err != nil {
		return nil, err
	}

	friendsUpstream, err := newUpstreamHandler(cfg.FriendsURL)
	if err != nil {
		return nil, err
	}

	websocketUpstream, err := newUpstreamHandler(cfg.WebsocketURL)
	if err != nil {
		return nil, err
	}

	chatUpstream, err := newUpstreamHandler(cfg.ChatURL)
	if err != nil {
		return nil, err
	}

	presenceUpstream, err := newUpstreamHandler(cfg.PresenceURL)
	if err != nil {
		return nil, err
	}

	loggingUpstream, err := newUpstreamHandler(cfg.LoggingURL)
	if err != nil {
		return nil, err
	}

	mux.Handle("GET /ws", jwtMiddleware.Wrap(websocketUpstream))
	mux.Handle("GET /messages/{userId}", jwtMiddleware.Wrap(chatUpstream))
	mux.Handle("GET /presence/{userId}", jwtMiddleware.Wrap(presenceUpstream))
	mux.Handle("GET /logs", jwtMiddleware.WrapAdmin(loggingUpstream))
	mux.Handle("GET /logs/ws", jwtMiddleware.WrapAdmin(loggingUpstream))
	mux.Handle("POST /auth/login", authUpstream)
	mux.Handle("POST /auth/register", authUpstream)
	mux.Handle("POST /friends/requests", jwtMiddleware.Wrap(friendsUpstream))
	mux.Handle("POST /friends/requests/{id}/accept", jwtMiddleware.Wrap(friendsUpstream))
	mux.Handle("DELETE /friends/requests/{id}/decline", jwtMiddleware.Wrap(friendsUpstream))
	mux.Handle("GET /friends/requests/pending", jwtMiddleware.Wrap(friendsUpstream))
	mux.Handle("GET /friends", jwtMiddleware.Wrap(friendsUpstream))
	mux.Handle("DELETE /friends/{friendId}", jwtMiddleware.Wrap(friendsUpstream))

	return withCORS(cfg.AllowedOrigin, mux), nil
}

func parseSecretList(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	secrets := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		secrets = append(secrets, part)
	}

	return secrets
}

func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
