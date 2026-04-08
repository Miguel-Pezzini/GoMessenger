package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type repositoryStub struct {
	presence    PresenceSnapshot
	presenceErr error
	publishedCh string
	published   NotificationMessage
}

func (r *repositoryStub) GetPresence(context.Context, string) (PresenceSnapshot, error) {
	return r.presence, r.presenceErr
}

func (r *repositoryStub) PublishNotification(_ context.Context, channel string, notification NotificationMessage) error {
	r.publishedCh = channel
	r.published = notification
	return nil
}

func TestHandleFriendRequestIntentPublishesNotification(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo, "notifications")

	err := service.HandleFriendRequestIntent(context.Background(), FriendRequestIntent{
		Type:            NotificationTypeFriendRequestReceived,
		EventID:         "req-1",
		SenderID:        "user-a",
		ReceiverID:      "user-b",
		FriendRequestID: "req-1",
		OccurredAt:      "2026-04-08T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	payload := decodePublishedPayload(t, repo)
	if repo.published.Type != MessageTypeNotification {
		t.Fatalf("expected message type %s, got %s", MessageTypeNotification, repo.published.Type)
	}
	if payload.NotificationType != NotificationTypeFriendRequestReceived {
		t.Fatalf("expected notification type %s, got %s", NotificationTypeFriendRequestReceived, payload.NotificationType)
	}
	if payload.RecipientUserID != "user-b" {
		t.Fatalf("expected recipient user-b, got %s", payload.RecipientUserID)
	}
	if payload.ActorUserID != "user-a" {
		t.Fatalf("expected actor user-a, got %s", payload.ActorUserID)
	}
	if payload.EntityID != "req-1" {
		t.Fatalf("expected entity req-1, got %s", payload.EntityID)
	}
}

func TestHandleMessageIntentPublishesWhenPresenceMissing(t *testing.T) {
	repo := &repositoryStub{presenceErr: ErrPresenceNotFound}
	service := NewService(repo, "notifications")

	published, err := service.HandleMessageIntent(context.Background(), validMessageIntent())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !published {
		t.Fatal("expected notification to be published")
	}

	payload := decodePublishedPayload(t, repo)
	if payload.NotificationType != NotificationTypeMessageReceived {
		t.Fatalf("expected notification type %s, got %s", NotificationTypeMessageReceived, payload.NotificationType)
	}
	if payload.Preview != "hello" {
		t.Fatalf("expected preview hello, got %s", payload.Preview)
	}
}

func TestHandleMessageIntentPublishesWhenUserOffline(t *testing.T) {
	repo := &repositoryStub{presence: PresenceSnapshot{UserID: "user-b", Status: "offline"}}
	service := NewService(repo, "notifications")

	published, err := service.HandleMessageIntent(context.Background(), validMessageIntent())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !published {
		t.Fatal("expected notification to be published")
	}
}

func TestHandleMessageIntentPublishesWhenUserInDifferentChat(t *testing.T) {
	repo := &repositoryStub{presence: PresenceSnapshot{UserID: "user-b", Status: PresenceStatusOnline, CurrentChatID: "user-c"}}
	service := NewService(repo, "notifications")

	published, err := service.HandleMessageIntent(context.Background(), validMessageIntent())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !published {
		t.Fatal("expected notification to be published")
	}
}

func TestHandleMessageIntentSuppressesWhenUserIsActiveInChat(t *testing.T) {
	repo := &repositoryStub{presence: PresenceSnapshot{UserID: "user-b", Status: PresenceStatusOnline, CurrentChatID: "user-a"}}
	service := NewService(repo, "notifications")

	published, err := service.HandleMessageIntent(context.Background(), validMessageIntent())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if published {
		t.Fatal("expected notification to be suppressed")
	}
	if repo.published.Type != "" {
		t.Fatalf("expected no publish call, got %+v", repo.published)
	}
}

func TestHandleMessageIntentReturnsPresenceLookupError(t *testing.T) {
	repo := &repositoryStub{presenceErr: errors.New("redis unavailable")}
	service := NewService(repo, "notifications")

	published, err := service.HandleMessageIntent(context.Background(), validMessageIntent())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if published {
		t.Fatal("expected published=false on presence error")
	}
}

func TestHandleMessageIntentRejectsInvalidPayload(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo, "notifications")

	_, err := service.HandleMessageIntent(context.Background(), MessageIntent{
		Type:       NotificationTypeMessageReceived,
		SenderID:   "user-a",
		ReceiverID: "user-b",
		OccurredAt: "2026-04-08T00:00:00Z",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func validMessageIntent() MessageIntent {
	return MessageIntent{
		Type:       NotificationTypeMessageReceived,
		EventID:    "msg-1",
		MessageID:  "msg-1",
		SenderID:   "user-a",
		ReceiverID: "user-b",
		Content:    "hello",
		OccurredAt: "2026-04-08T00:00:00Z",
	}
}

func decodePublishedPayload(t *testing.T, repo *repositoryStub) NotificationPayload {
	t.Helper()

	var payload NotificationPayload
	if err := json.Unmarshal(repo.published.Payload, &payload); err != nil {
		t.Fatalf("failed to decode notification payload: %v", err)
	}
	return payload
}
