package presence

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
)

type Config struct {
	Address                string
	RedisAddr              string
	AuthUpstreamURL        string
	InternalServiceToken   string
	LifecycleEventsChannel string
	PresenceUpdatesChannel string
	RedisKeyPrefix         string
}

type Server struct {
	addr                   string
	lifecycleEventsChannel string
	service                *Service
	repo                   Repository
	handler                *Handler
}

func LoadConfig() Config {
	return Config{
		Address:                config.MustString("PRESENCE_ADDR"),
		RedisAddr:              config.MustString("REDIS_ADDR"),
		AuthUpstreamURL:        config.MustString("AUTH_UPSTREAM_URL"),
		InternalServiceToken:   config.String("INTERNAL_SERVICE_TOKEN", "dev-internal-token"),
		LifecycleEventsChannel: config.MustString("REDIS_CHANNEL_PRESENCE_EVENTS"),
		PresenceUpdatesChannel: config.MustString("REDIS_CHANNEL_PRESENCE"),
		RedisKeyPrefix:         config.MustString("REDIS_KEY_PREFIX_PRESENCE"),
	}
}

func NewServer(cfg Config) (*Server, error) {
	rdb, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return nil, err
	}

	repo := NewRedisRepository(rdb, cfg.RedisKeyPrefix)
	authLookup := NewAuthHTTPClient(cfg.AuthUpstreamURL, cfg.InternalServiceToken)
	service := NewService(repo, cfg.PresenceUpdatesChannel, authLookup)

	return &Server{
		addr:                   cfg.Address,
		lifecycleEventsChannel: cfg.LifecycleEventsChannel,
		service:                service,
		repo:                   repo,
		handler:                NewHandler(service),
	}, nil
}

func (s *Server) Start() error {
	ctx := context.Background()

	go func() {
		err := s.repo.Subscribe(ctx, s.lifecycleEventsChannel, func(event LifecycleEvent) {
			if _, handleErr := s.service.HandleLifecycleEvent(ctx, event); handleErr != nil {
				log.Printf("presence: failed to handle lifecycle event: %v", handleErr)
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("presence: lifecycle subscription stopped: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("GET /presence/{userID}", http.HandlerFunc(s.handler.HandleGetPresence))
	mux.Handle("GET /admin/presence/active", http.HandlerFunc(s.handler.HandleListActiveUsers))
	return http.ListenAndServe(s.addr, mux)
}
