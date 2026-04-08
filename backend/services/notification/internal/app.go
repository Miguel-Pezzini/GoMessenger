package notification

import (
	"context"
	"log"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
)

type Config struct {
	Address              string
	RedisAddr            string
	FriendStream         string
	FriendConsumerGroup  string
	MessageStream        string
	MessageConsumerGroup string
	NotificationChannel  string
	PresenceKeyPrefix    string
}

func LoadConfig() Config {
	return Config{
		Address:              config.MustString("NOTIFICATION_ADDR"),
		RedisAddr:            config.MustString("REDIS_ADDR"),
		FriendStream:         config.MustString("REDIS_STREAM_NOTIFICATION_FRIEND_REQUESTS"),
		FriendConsumerGroup:  config.MustString("NOTIFICATION_FRIEND_CONSUMER_GROUP"),
		MessageStream:        config.MustString("REDIS_STREAM_NOTIFICATION_MESSAGES"),
		MessageConsumerGroup: config.MustString("NOTIFICATION_MESSAGE_CONSUMER_GROUP"),
		NotificationChannel:  config.MustString("REDIS_CHANNEL_NOTIFICATIONS"),
		PresenceKeyPrefix:    config.MustString("REDIS_KEY_PREFIX_PRESENCE"),
	}
}

func Run() error {
	cfg := LoadConfig()

	rdb, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return err
	}
	defer rdb.Close()

	repo := NewRedisRepository(rdb, cfg.PresenceKeyPrefix)
	service := NewService(repo, cfg.NotificationChannel)
	server := NewStreamServer(rdb, service, cfg.FriendStream, cfg.FriendConsumerGroup, cfg.MessageStream, cfg.MessageConsumerGroup, cfg.Address)

	ctx := context.Background()
	go func() {
		if err := server.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("notification: stream server stopped: %v", err)
		}
	}()

	log.Printf("notification service listening on %s", cfg.Address)
	return http.ListenAndServe(cfg.Address, http.NewServeMux())
}
