package websocket

import (
	"net/http"
	"strings"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/security"
)

type Server struct {
	addr                  string
	channelName           string
	friendEventsChannel   string
	presenceEventsChannel string
	chatEventsChannel     string
	notificationsChannel  string
	handler               *Handler
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
	}
}

func Run() error {
	cfg := LoadConfig()

	redisClient, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return err
	}

	service := NewService(NewRedisRepository(redisClient), cfg.StreamName)
	server := NewServer(cfg.Address, cfg.ChannelName, cfg.FriendEventsChannel, cfg.PresenceEventsChannel, cfg.ChatEventsChannel, cfg.NotificationsChannel, NewHandler(service, audit.NewRedisPublisher(redisClient, cfg.AuditStream), security.NewOriginValidator(cfg.AllowedOrigins)))
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

func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.handler.SetPresenceChannel(s.presenceEventsChannel)
	s.handler.SetChatEventsChannel(s.chatEventsChannel)
	s.handler.StartPubSubListener(s.channelName)
	s.handler.StartFriendEventListener(s.friendEventsChannel)
	s.handler.StartChatEventListener(s.chatEventsChannel)
	s.handler.StartNotificationListener(s.notificationsChannel)
	mux.Handle("GET /ws", http.HandlerFunc(s.handler.HandleConnection))
	return http.ListenAndServe(s.addr, mux)
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
