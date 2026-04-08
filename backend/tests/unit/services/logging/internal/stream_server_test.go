package logging

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type streamRepositoryStub struct {
	events []StoredEvent
}

func (r *streamRepositoryStub) Append(_ context.Context, event StoredEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *streamRepositoryStub) ListRecent(_ context.Context, _ int) ([]StoredEvent, error) {
	return append([]StoredEvent(nil), r.events...), nil
}

func TestDecodeEventRejectsMissingPayload(t *testing.T) {
	if _, err := decodeEvent(redis.XMessage{ID: "1-0"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessMessageAcknowledgesValidEvent(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	service := NewService(&streamRepositoryStub{})
	server := NewServer("audit.logs", newRedisClient(fakeRedis.addr()), service)
	defer server.rdb.Close()

	err := server.processMessage(context.Background(), redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"event_id":"evt-1","event_type":"user.registered","category":"audit","service":"auth","occurred_at":"` + time.Now().UTC().Format(time.RFC3339) + `","status":"success","message":"user registered"}`,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.ackCount != 1 {
		t.Fatalf("expected one ack, got %d", fakeRedis.ackCount)
	}
}
