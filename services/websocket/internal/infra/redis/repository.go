package redis

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	rdb *redis.Client
}

func NewRedisRepository(rdb *redis.Client) *RedisRepository {
	return &RedisRepository{rdb: rdb}
}

func (r *RedisRepository) AddToStream(streamName, payload string) error {
	_, err := r.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{"payload": payload},
	}).Result()

	return err
}

func (r *RedisRepository) Subscribe(channelName string, handler func(payload string)) {
	pubsub := r.rdb.Subscribe(context.Background(), channelName)
	_, err := pubsub.Receive(context.Background())
	if err != nil {
		log.Println("Erro ao subscribir no canal", channelName, err)
		return
	}

	ch := pubsub.Channel()
	for msg := range ch {
		handler(msg.Payload)
	}
}
