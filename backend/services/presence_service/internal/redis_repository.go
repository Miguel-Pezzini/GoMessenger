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

func (r *RedisRepository) ListActive(ctx context.Context, limit int) ([]Presence, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	var cursor uint64
	active := make([]Presence, 0, limit)
	for {
		keys, nextCursor, err := r.rdb.Scan(ctx, cursor, r.keyPrefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			payload, err := r.rdb.Get(ctx, key).Bytes()
			if errors.Is(err, redisv9.Nil) {
				continue
			}
			if err != nil {
				return nil, err
			}

			var presence Presence
			if err := json.Unmarshal(payload, &presence); err != nil {
				log.Printf("presence: invalid stored presence at %s: %v", key, err)
				continue
			}
			if presence.Status != StatusOnline {
				continue
			}

			active = append(active, presence)
			if len(active) >= limit {
				return active, nil
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			return active, nil
		}
	}
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
