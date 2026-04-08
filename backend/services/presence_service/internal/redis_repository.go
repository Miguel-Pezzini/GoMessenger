package presence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	redisv9 "github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	rdb       *redisv9.Client
	keyPrefix string
}

func NewRedisRepository(rdb *redisv9.Client, keyPrefix string) *RedisRepository {
	return &RedisRepository{rdb: rdb, keyPrefix: keyPrefix}
}

func (r *RedisRepository) Save(ctx context.Context, presence Presence) error {
	payload, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	return r.rdb.Set(ctx, r.key(presence.UserID), payload, 0).Err()
}

func (r *RedisRepository) Get(ctx context.Context, userID string) (Presence, error) {
	payload, err := r.rdb.Get(ctx, r.key(userID)).Bytes()
	if errors.Is(err, redisv9.Nil) {
		return Presence{}, ErrPresenceNotFound
	}
	if err != nil {
		return Presence{}, err
	}

	var presence Presence
	if err := json.Unmarshal(payload, &presence); err != nil {
		return Presence{}, err
	}

	return presence, nil
}

func (r *RedisRepository) Publish(ctx context.Context, channel string, presence Presence) error {
	payload, err := json.Marshal(presence)
	if err != nil {
		return err
	}

	return r.rdb.Publish(ctx, channel, payload).Err()
}

func (r *RedisRepository) Subscribe(ctx context.Context, channel string, handler func(LifecycleEvent)) error {
	pubsub := r.rdb.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("subscribe %s: %w", channel, err)
	}

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			_ = pubsub.Close()
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			var event LifecycleEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("presence: invalid lifecycle payload: %v", err)
				continue
			}
			handler(event)
		}
	}
}

func (r *RedisRepository) key(userID string) string {
	return r.keyPrefix + userID
}
