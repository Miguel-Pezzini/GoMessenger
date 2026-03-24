package app

import (
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
	"github.com/Miguel-Pezzini/GoMessenger/services/websocket/internal/domain"
	infraredis "github.com/Miguel-Pezzini/GoMessenger/services/websocket/internal/infra/redis"
	httptransport "github.com/Miguel-Pezzini/GoMessenger/services/websocket/internal/transport/http"
)

type Server struct {
	addr        string
	channelName string
	handler     *httptransport.Handler
}

type Config struct {
	Address     string
	RedisAddr   string
	StreamName  string
	ChannelName string
}

func LoadConfig() Config {
	return Config{
		Address:     config.String("WEBSOCKET_ADDR", ":8081"),
		RedisAddr:   config.String("REDIS_ADDR", "localhost:6379"),
		StreamName:  config.String("REDIS_STREAM_CHAT", "chat.message.created"),
		ChannelName: config.String("REDIS_CHANNEL_CHAT", "chat.message.persisted"),
	}
}

func Run() error {
	cfg := LoadConfig()

	redisClient, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return err
	}

	service := domain.NewService(infraredis.NewRedisRepository(redisClient), cfg.StreamName)
	server := NewServer(cfg.Address, cfg.ChannelName, httptransport.NewHandler(service))
	return server.Start()
}

func NewServer(addr, channelName string, handler *httptransport.Handler) *Server {
	return &Server{
		addr:        addr,
		channelName: channelName,
		handler:     handler,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.handler.StartPubSubListener(s.channelName)
	mux.Handle("GET /ws", http.HandlerFunc(s.handler.HandleConnection))
	return http.ListenAndServe(s.addr, mux)
}
