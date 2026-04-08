package logging

import (
	"context"
	"sync"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
)

type Service struct {
	repo        Repository
	mu          sync.RWMutex
	subscribers map[chan StoredEvent]struct{}
}

func NewService(repo Repository) *Service {
	return &Service{
		repo:        repo,
		subscribers: make(map[chan StoredEvent]struct{}),
	}
}

func (s *Service) Ingest(ctx context.Context, streamID string, event audit.Event) (StoredEvent, error) {
	stored := StoredEvent{
		StreamID: streamID,
		Event:    event.Normalize(),
	}
	if err := stored.Event.Validate(); err != nil {
		return StoredEvent{}, err
	}
	if err := s.repo.Append(ctx, stored); err != nil {
		return StoredEvent{}, err
	}
	s.broadcast(stored)
	return stored, nil
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]StoredEvent, error) {
	return s.repo.ListRecent(ctx, limit)
}

func (s *Service) Subscribe() (<-chan StoredEvent, func()) {
	ch := make(chan StoredEvent, 16)

	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	unsubscribe := func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}

	return ch, unsubscribe
}

func (s *Service) broadcast(event StoredEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
