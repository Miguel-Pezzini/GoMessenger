package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisPublisher struct {
	rdb        *redis.Client
	streamName string
}

func NewRedisPublisher(rdb *redis.Client, streamName string) *RedisPublisher {
	return &RedisPublisher{
		rdb:        rdb,
		streamName: streamName,
	}
}

func (p *RedisPublisher) Publish(ctx context.Context, event Event) error {
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	if err := p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: p.streamName,
		Values: map[string]any{"payload": string(payload)},
	}).Err(); err != nil {
		return fmt.Errorf("publish audit event: %w", err)
	}

	return nil
}
