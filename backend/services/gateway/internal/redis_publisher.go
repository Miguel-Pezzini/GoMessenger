package gateway

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

type Publisher struct {
	rdb *redis.Client
}

func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

func (p *Publisher) Publish(channel string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("redis publisher: failed to marshal payload:", err)
		return
	}
	if err := p.rdb.Publish(context.Background(), channel, string(data)).Err(); err != nil {
		log.Println("redis publisher: failed to publish to channel", channel, ":", err)
	}
}
