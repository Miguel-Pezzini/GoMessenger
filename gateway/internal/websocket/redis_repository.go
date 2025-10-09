package websocket

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	rdb *redis.Client
	ctx context.Context
}

func NewRedisRepository(rdb *redis.Client) *RedisRepository {
	return &RedisRepository{
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (r *RedisRepository) Publish(channel string, message string) error {
	return r.rdb.Publish(r.ctx, channel, message).Err()
}

func (r *RedisRepository) Subscribe(channel string, handler func(string)) {
	pubsub := r.rdb.Subscribe(r.ctx, channel)
	ch := pubsub.Channel()

	go func() {
		for msg := range ch {
			handler(msg.Payload)
		}
	}()
}

func (r *RedisRepository) SetSession(userID, gatewayID string) error {
	return r.rdb.Set(r.ctx, "session:"+userID, gatewayID, 0).Err()
}

func (r *RedisRepository) GetSession(userID string) (string, error) {
	return r.rdb.Get(r.ctx, "session:"+userID).Result()
}
