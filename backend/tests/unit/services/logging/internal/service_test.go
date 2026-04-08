package logging

import (
	"context"
	"testing"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
)

type serviceRepositoryStub struct {
	events []StoredEvent
	err    error
}

func (r *serviceRepositoryStub) Append(_ context.Context, event StoredEvent) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, event)
	return nil
}

func (r *serviceRepositoryStub) ListRecent(_ context.Context, limit int) ([]StoredEvent, error) {
	if limit > len(r.events) {
		limit = len(r.events)
	}
	return append([]StoredEvent(nil), r.events[:limit]...), nil
}

func TestIngestAppendsAndBroadcasts(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo)
	ch, unsubscribe := service.Subscribe()
	defer unsubscribe()

	stored, err := service.Ingest(context.Background(), "1-0", audit.Event{
		EventType:  "user.registered",
		Category:   audit.CategoryAudit,
		Service:    "auth",
		Status:     audit.StatusSuccess,
		Message:    "user registered",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("ingest returned error: %v", err)
	}
	if len(repo.events) != 1 {
		t.Fatalf("expected one stored event, got %d", len(repo.events))
	}
	select {
	case got := <-ch:
		if got.StreamID != stored.StreamID {
			t.Fatalf("expected stream id %s, got %s", stored.StreamID, got.StreamID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected broadcast to subscriber")
	}
}

func TestIngestRejectsInvalidEvent(t *testing.T) {
	service := NewService(&serviceRepositoryStub{})
	if _, err := service.Ingest(context.Background(), "1-0", audit.Event{}); err == nil {
		t.Fatal("expected validation error")
	}
}
