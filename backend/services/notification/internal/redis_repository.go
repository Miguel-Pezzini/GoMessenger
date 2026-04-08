package notification

import (
	"context"
	"encoding/json"
	"errors"

	redisv9 "github.com/redis/go-redis/v9"
)

var ErrPresenceNotFound = errors.New("presence not found")

type RedisRepository struct {
	rdb       *redisv9.Client
	keyPrefix string
}

func NewRedisRepository(rdb *redisv9.Client, keyPrefix string) *RedisRepository {
	return &RedisRepository{rdb: rdb, keyPrefix: keyPrefix}
}

func (r *RedisRepository) GetPresence(ctx context.Context, userID string) (PresenceSnapshot, error) {
	payload, err := r.rdb.Get(ctx, r.keyPrefix+userID).Bytes()
	if errors.Is(err, redisv9.Nil) {
		return PresenceSnapshot{}, ErrPresenceNotFound
	}
	if err != nil {
		return PresenceSnapshot{}, err
	}

	var presence PresenceSnapshot
	if err := json.Unmarshal(payload, &presence); err != nil {
		return PresenceSnapshot{}, err
	}

	return presence, nil
}

func (r *RedisRepository) PublishNotification(ctx context.Context, channel string, notification NotificationMessage) error {
	payload, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	return r.rdb.Publish(ctx, channel, payload).Err()
}
