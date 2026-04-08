package presence

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repositoryStub struct {
	saved             Presence
	publishedChannel  string
	publishedPresence Presence
	getPresence       Presence
	saveErr           error
	getErr            error
	publishErr        error
}

func (r *repositoryStub) Save(_ context.Context, presence Presence) error {
	r.saved = presence
	return r.saveErr
}

func (r *repositoryStub) Get(_ context.Context, _ string) (Presence, error) {
	return r.getPresence, r.getErr
}

func (r *repositoryStub) Publish(_ context.Context, channel string, presence Presence) error {
	r.publishedChannel = channel
	r.publishedPresence = presence
	return r.publishErr
}

func (r *repositoryStub) Subscribe(_ context.Context, _ string, _ func(LifecycleEvent)) error {
	return nil
}

func TestHandleLifecycleEvent(t *testing.T) {
	disconnectedAt := time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		event    LifecycleEvent
		expected Presence
	}{
		{
			name: "connected marks user online",
			event: LifecycleEvent{
				UserID:        "user-a",
				Type:          LifecycleEventConnected,
				CurrentChatID: "chat-1",
			},
			expected: Presence{
				UserID:        "user-a",
				Status:        StatusOnline,
				CurrentChatID: "chat-1",
			},
		},
		{
			name: "chat opened keeps user online and stores current chat",
			event: LifecycleEvent{
				UserID:        "user-a",
				Type:          LifecycleEventChatOpened,
				CurrentChatID: "chat-1",
			},
			expected: Presence{
				UserID:        "user-a",
				Status:        StatusOnline,
				CurrentChatID: "chat-1",
			},
		},
		{
			name: "chat closed keeps user online and clears current chat",
			event: LifecycleEvent{
				UserID:        "user-a",
				Type:          LifecycleEventChatClosed,
				CurrentChatID: "chat-1",
			},
			expected: Presence{
				UserID: "user-a",
				Status: StatusOnline,
			},
		},
		{
			name: "disconnected marks user offline and stores last seen",
			event: LifecycleEvent{
				UserID:        "user-a",
				Type:          LifecycleEventDisconnected,
				CurrentChatID: "chat-1",
				OccurredAt:    disconnectedAt,
			},
			expected: Presence{
				UserID:   "user-a",
				Status:   StatusOffline,
				LastSeen: &disconnectedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &repositoryStub{}
			service := NewService(repo, "presence.updated")

			got, err := service.HandleLifecycleEvent(context.Background(), tt.event)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if repo.publishedChannel != "presence.updated" {
				t.Fatalf("expected publish channel presence.updated, got %s", repo.publishedChannel)
			}

			assertPresenceEqual(t, tt.expected, got)
			assertPresenceEqual(t, tt.expected, repo.saved)
			assertPresenceEqual(t, tt.expected, repo.publishedPresence)
		})
	}
}

func TestHandleLifecycleEventRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		event LifecycleEvent
	}{
		{
			name: "missing user id",
			event: LifecycleEvent{
				Type: LifecycleEventConnected,
			},
		},
		{
			name: "unsupported type",
			event: LifecycleEvent{
				UserID: "user-a",
				Type:   "unknown",
			},
		},
		{
			name: "disconnect requires occurred at",
			event: LifecycleEvent{
				UserID: "user-a",
				Type:   LifecycleEventDisconnected,
			},
		},
		{
			name: "chat opened requires current chat id",
			event: LifecycleEvent{
				UserID: "user-a",
				Type:   LifecycleEventChatOpened,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &repositoryStub{}
			service := NewService(repo, "presence.updated")

			if _, err := service.HandleLifecycleEvent(context.Background(), tt.event); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestGetPresence(t *testing.T) {
	now := time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)
	expected := Presence{
		UserID:        "user-a",
		Status:        StatusOffline,
		LastSeen:      &now,
		CurrentChatID: "chat-1",
	}

	repo := &repositoryStub{getPresence: expected}
	service := NewService(repo, "presence.updated")

	got, err := service.GetPresence(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	assertPresenceEqual(t, expected, got)
}

func TestGetPresenceValidatesUserID(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo, "presence.updated")

	if _, err := service.GetPresence(context.Background(), ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetPresencePropagatesRepositoryError(t *testing.T) {
	expectedErr := errors.New("boom")
	repo := &repositoryStub{getErr: expectedErr}
	service := NewService(repo, "presence.updated")

	_, err := service.GetPresence(context.Background(), "user-a")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func assertPresenceEqual(t *testing.T, expected, got Presence) {
	t.Helper()

	if expected.UserID != got.UserID {
		t.Fatalf("expected user_id %s, got %s", expected.UserID, got.UserID)
	}
	if expected.Status != got.Status {
		t.Fatalf("expected status %s, got %s", expected.Status, got.Status)
	}
	if expected.CurrentChatID != got.CurrentChatID {
		t.Fatalf("expected current_chat_id %s, got %s", expected.CurrentChatID, got.CurrentChatID)
	}

	switch {
	case expected.LastSeen == nil && got.LastSeen == nil:
		return
	case expected.LastSeen == nil || got.LastSeen == nil:
		t.Fatalf("expected last_seen %v, got %v", expected.LastSeen, got.LastSeen)
	case !expected.LastSeen.Equal(*got.LastSeen):
		t.Fatalf("expected last_seen %v, got %v", expected.LastSeen, got.LastSeen)
	}
}
