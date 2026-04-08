package chat

import (
	"log"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
)

type Config struct {
	Address                  string
	MongoURI                 string
	MongoDatabase            string
	RedisAddr                string
	RedisStreamChat          string
	RedisChannelChat         string
	RedisChannelChatEvents   string
	RedisStreamNotifications string
	RedisStreamAudit         string
}

func LoadConfig() Config {
	return Config{
		Address:                  config.MustString("CHAT_ADDR"),
		MongoURI:                 config.MustString("CHAT_MONGO_URI"),
		MongoDatabase:            config.MustString("CHAT_MONGO_DB"),
		RedisAddr:                config.MustString("REDIS_ADDR"),
		RedisStreamChat:          config.MustString("REDIS_STREAM_CHAT"),
		RedisChannelChat:         config.MustString("REDIS_CHANNEL_CHAT"),
		RedisChannelChatEvents:   config.MustString("REDIS_CHANNEL_CHAT_EVENTS"),
		RedisStreamNotifications: config.MustString("REDIS_STREAM_NOTIFICATION_MESSAGES"),
		RedisStreamAudit:         config.MustString("REDIS_STREAM_AUDIT_LOGS"),
	}
}

func Run() error {
	cfg := LoadConfig()

	db, err := mongoutil.NewDatabase(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return err
	}
	rdb, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return err
	}

	repo, err := NewMongoRepository(db)
	if err != nil {
		return err
	}

	service := NewService(repo)

	// HTTP server for chat history queries
	mux := http.NewServeMux()
	handler := NewHandler(service)
	mux.HandleFunc("GET /messages/{userId}", handler.GetConversation)

	errCh := make(chan error, 1)

	go func() {
		log.Printf("chat HTTP listening on %s", cfg.Address)
		errCh <- http.ListenAndServe(cfg.Address, mux)
	}()

	// Stream consumer — runs in the foreground; any error is fatal
	go func() {
		server := NewServer(cfg.Address, cfg.RedisStreamChat, cfg.RedisChannelChat, cfg.RedisChannelChatEvents, cfg.RedisStreamNotifications, rdb, service, audit.NewRedisPublisher(rdb, cfg.RedisStreamAudit))
		errCh <- server.Start()
	}()

	return <-errCh
}
