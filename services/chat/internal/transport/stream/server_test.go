package stream

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestDecodeMessage(t *testing.T) {
	msg := redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"sender_id":"user-a","receiver_id":"user-b","content":"hello","timestamp":123}`,
		},
	}

	req, err := decodeMessage(msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if req.SenderID != "user-a" {
		t.Fatalf("expected sender user-a, got %s", req.SenderID)
	}
	if req.ReceiverID != "user-b" {
		t.Fatalf("expected receiver user-b, got %s", req.ReceiverID)
	}
}

func TestDecodeMessageRejectsMissingPayload(t *testing.T) {
	_, err := decodeMessage(redis.XMessage{
		ID:     "1-0",
		Values: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
