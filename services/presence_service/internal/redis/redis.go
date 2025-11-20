package redisutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("falha ao conectar ao Redis: %w", err)
	}

	return client, nil
}

type RedisConfig struct {
	StreamChat  string
	ChannelChat string
}

func LoadRedisConfig() *RedisConfig {
	return &RedisConfig{
		ChannelChat: os.Getenv("REDIS_CHANNEL_PRESENCE"),
	}
}
