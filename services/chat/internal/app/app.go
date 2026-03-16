package app

import (
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
	"github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/domain"
	mongorepo "github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/infra/mongo"
	streamtransport "github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/transport/stream"
)

type Config struct {
	Address          string
	MongoURI         string
	MongoDatabase    string
	RedisAddr        string
	RedisStreamChat  string
	RedisChannelChat string
}

func LoadConfig() Config {
	return Config{
		Address:          config.String("CHAT_ADDR", ":8081"),
		MongoURI:         config.String("CHAT_MONGO_URI", "mongodb://localhost:27018"),
		MongoDatabase:    config.String("CHAT_MONGO_DB", "chatdb"),
		RedisAddr:        config.String("REDIS_ADDR", "localhost:6379"),
		RedisStreamChat:  config.String("REDIS_STREAM_CHAT", "chat.message.created"),
		RedisChannelChat: config.String("REDIS_CHANNEL_CHAT", "chat.message.persisted"),
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

	service := domain.NewService(mongorepo.NewRepository(db))
	server := streamtransport.NewServer(cfg.Address, cfg.RedisStreamChat, cfg.RedisChannelChat, rdb, service)
	return server.Start()
}
