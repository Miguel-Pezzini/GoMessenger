package audit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisPublisherRejectsInvalidEvent(t *testing.T) {
	publisher := NewRedisPublisher(redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:0",
	}), "audit.logs")

	err := publisher.Publish(context.Background(), Event{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRedisPublisherPublishesValidEvent(t *testing.T) {
	server := newFakeRedisServer(t)
	defer server.close()

	client := redis.NewClient(&redis.Options{
		Addr:         server.addr(),
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		MaxRetries:   0,
	})
	defer client.Close()

	publisher := NewRedisPublisher(client, "audit.logs")
	err := publisher.Publish(context.Background(), Event{
		EventType:  "user.registered",
		Category:   CategoryAudit,
		Service:    "auth",
		Status:     StatusSuccess,
		Message:    "user registered",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("publish returned error: %v", err)
	}
	if server.xaddCount != 1 {
		t.Fatalf("expected one xadd, got %d", server.xaddCount)
	}
}
