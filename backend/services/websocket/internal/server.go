package websocket

import (
	"net/http"
	"strings"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/observability"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/security"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	addr                  string
	channelName           string
	friendEventsChannel   string
	presenceEventsChannel string
	chatEventsChannel     string
	notificationsChannel  string
	handler               *Handler
	observer              *observability.Observer
	rdb                   *redis.Client
	mediaInternalURL      string
}

type Config struct {
	Address               string
	RedisAddr             string
	StreamName            string
	ChannelName           string
	FriendEventsChannel   string
	PresenceEventsChannel string
	ChatEventsChannel     string
	NotificationsChannel  string
	AuditStream           string
	AllowedOrigins        []string
	MediaInternalURL      string
	InternalServiceToken  string
}

func LoadConfig() Config {
	return Config{
		Address:               config.MustString("WEBSOCKET_ADDR"),
		RedisAddr:             config.MustString("REDIS_ADDR"),
		StreamName:            config.MustString("REDIS_STREAM_CHAT"),
		ChannelName:           config.MustString("REDIS_CHANNEL_CHAT"),
		FriendEventsChannel:   config.MustString("REDIS_CHANNEL_FRIEND_EVENTS"),
		PresenceEventsChannel: config.MustString("REDIS_CHANNEL_PRESENCE_EVENTS"),
		ChatEventsChannel:     config.MustString("REDIS_CHANNEL_CHAT_EVENTS"),
		NotificationsChannel:  config.MustString("REDIS_CHANNEL_NOTIFICATIONS"),
		AuditStream:           config.MustString("REDIS_STREAM_AUDIT_LOGS"),
		AllowedOrigins:        parseAllowedOrigins(config.String("WEBSOCKET_ALLOWED_ORIGINS", config.String("GATEWAY_ALLOWED_ORIGIN", ""))),
		MediaInternalURL:      config.MustString("MEDIA_INTERNAL_URL"),
		InternalServiceToken:  config.String("INTERNAL_SERVICE_TOKEN", "dev-internal-token"),
	}
}

func Run() error {
	cfg := LoadConfig()

	redisClient, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return err
	}

	observer := observability.New("websocket")
	service := NewService(NewRedisRepository(redisClient), cfg.StreamName, NewMediaHTTPClient(cfg.MediaInternalURL, cfg.InternalServiceToken))
	server := NewServer(cfg.Address, cfg.ChannelName, cfg.FriendEventsChannel, cfg.PresenceEventsChannel, cfg.ChatEventsChannel, cfg.NotificationsChannel, NewHandler(service, audit.NewRedisPublisher(redisClient, cfg.AuditStream), security.NewOriginValidator(cfg.AllowedOrigins), observer))
	server.SetObserver(observer)
	server.SetReadinessDependencies(redisClient, cfg.MediaInternalURL)
	return server.Start()
}

func NewServer(addr, channelName, friendEventsChannel, presenceEventsChannel, chatEventsChannel, notificationsChannel string, handler *Handler) *Server {
	return &Server{
		addr:                  addr,
		channelName:           channelName,
		friendEventsChannel:   friendEventsChannel,
		presenceEventsChannel: presenceEventsChannel,
		chatEventsChannel:     chatEventsChannel,
		notificationsChannel:  notificationsChannel,
		handler:               handler,
	}
}

func (s *Server) SetObserver(observer *observability.Observer) {
	s.observer = observer
}

func (s *Server) SetReadinessDependencies(rdb *redis.Client, mediaInternalURL string) {
	s.rdb = rdb
	s.mediaInternalURL = mediaInternalURL
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.handler.SetPresenceChannel(s.presenceEventsChannel)
	s.handler.SetChatEventsChannel(s.chatEventsChannel)
	s.handler.StartPubSubListener(s.channelName)
	s.handler.StartFriendEventListener(s.friendEventsChannel)
	s.handler.StartChatEventListener(s.chatEventsChannel)
	s.handler.StartNotificationListener(s.notificationsChannel)
	mux.Handle("GET /ws", http.HandlerFunc(s.handler.HandleConnection))

	if s.observer == nil {
		s.observer = observability.New("websocket")
	}
	checks := []observability.ReadinessCheck{}
	if s.rdb != nil {
		checks = append(checks, observability.RedisPingCheck("redis", s.rdb))
	}
	if s.mediaInternalURL != "" {
		checks = append(checks, observability.HTTPHealthCheck("media", s.mediaInternalURL))
	}
	s.observer.Mount(mux, checks...)
	return http.ListenAndServe(s.addr, s.observer.Handler(mux))
}

func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		origins = append(origins, part)
	}

	return origins
}
