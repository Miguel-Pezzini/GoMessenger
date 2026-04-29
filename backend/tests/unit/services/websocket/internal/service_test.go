package websocket

import (
	"encoding/json"
	"testing"
)

type serviceRepositoryStub struct {
	streamName string
	payload    string
	channel    string
	addErr     error
	publishErr error
}

func (r *serviceRepositoryStub) AddToStream(streamName, payload string) error {
	r.streamName = streamName
	r.payload = payload
	return r.addErr
}

func (r *serviceRepositoryStub) Publish(channelName, payload string) error {
	r.channel = channelName
	r.payload = payload
	return r.publishErr
}

func (r *serviceRepositoryStub) Subscribe(_ string, _ func(payload string)) {}

type mediaPreparerStub struct {
	senderID      string
	receiverID    string
	attachmentIDs []string
	attachments   []AttachmentSnapshot
	err           error
}

func (m *mediaPreparerStub) PrepareMessage(senderID, receiverID string, attachmentIDs []string) ([]AttachmentSnapshot, error) {
	m.senderID = senderID
	m.receiverID = receiverID
	m.attachmentIDs = attachmentIDs
	if m.err != nil {
		return nil, m.err
	}
	return m.attachments, nil
}

func TestPersistMessageUsesAuthenticatedUserAsSender(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	err := service.PersistMessage("user-auth", ChatMessagePayload{
		ReceiverID: "user-b",
		Content:    "hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload ChatStreamPayload
	if err := json.Unmarshal([]byte(repo.payload), &payload); err != nil {
		t.Fatalf("failed to decode stream payload: %v", err)
	}
	if payload.SenderID != "user-auth" || payload.ReceiverID != "user-b" || payload.Content != "hello" {
		t.Fatalf("unexpected stream payload: %+v", payload)
	}
	if payload.Timestamp == 0 {
		t.Fatal("expected stream payload timestamp to be populated")
	}
	if repo.streamName != "chat-stream" {
		t.Fatalf("expected stream chat-stream, got %s", repo.streamName)
	}
}

func TestPersistMessageRejectsMismatchedSenderID(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	err := service.PersistMessage("user-auth", ChatMessagePayload{
		SenderID:   "user-other",
		ReceiverID: "user-b",
		Content:    "hello",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Message != "sender_id does not match authenticated user" {
		t.Fatalf("unexpected validation message: %s", validationErr.Message)
	}
	if repo.payload != "" {
		t.Fatalf("expected message not to be persisted, got payload %s", repo.payload)
	}
}

func TestPersistMessageRejectsMissingReceiverID(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	err := service.PersistMessage("user-auth", ChatMessagePayload{
		Content: "hello",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "receiver_id is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPersistMessageRejectsBlankContentWithoutAttachments(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	err := service.PersistMessage("user-auth", ChatMessagePayload{
		ReceiverID: "user-b",
		Content:    "   ",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if err.Error() != "content or attachment_ids is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPersistMessageAllowsAttachmentOnlyMessage(t *testing.T) {
	repo := &serviceRepositoryStub{}
	media := &mediaPreparerStub{
		attachments: []AttachmentSnapshot{{
			ID:          "attachment-1",
			Filename:    "photo.jpg",
			ContentType: "image/jpeg",
			Size:        42,
			Kind:        "image",
			DownloadURL: "/attachments/attachment-1",
		}},
	}
	service := NewService(repo, "chat-stream", media)

	err := service.PersistMessage("user-auth", ChatMessagePayload{
		ReceiverID:    " user-b ",
		AttachmentIDs: []string{" attachment-1 "},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if media.senderID != "user-auth" || media.receiverID != "user-b" {
		t.Fatalf("unexpected media prepare users: %s -> %s", media.senderID, media.receiverID)
	}
	if len(media.attachmentIDs) != 1 || media.attachmentIDs[0] != "attachment-1" {
		t.Fatalf("unexpected attachment ids: %+v", media.attachmentIDs)
	}

	var payload ChatStreamPayload
	if err := json.Unmarshal([]byte(repo.payload), &payload); err != nil {
		t.Fatalf("failed to decode stream payload: %v", err)
	}
	if payload.Content != "" {
		t.Fatalf("expected empty content, got %q", payload.Content)
	}
	if len(payload.Attachments) != 1 || payload.Attachments[0].ID != "attachment-1" {
		t.Fatalf("expected prepared attachment in payload, got %+v", payload.Attachments)
	}
}

func TestPublishPresenceConnected(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	if err := service.PublishPresenceConnected("presence.lifecycle", "user-auth"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.channel != "presence.lifecycle" {
		t.Fatalf("expected channel presence.lifecycle, got %s", repo.channel)
	}

	var event PresenceLifecycleEvent
	if err := json.Unmarshal([]byte(repo.payload), &event); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}
	if event.UserID != "user-auth" {
		t.Fatalf("expected user-auth, got %s", event.UserID)
	}
	if event.Type != "connected" {
		t.Fatalf("expected connected event, got %s", event.Type)
	}
	if event.OccurredAt == "" {
		t.Fatal("expected occurred_at to be set")
	}
}

func TestPublishPresenceDisconnected(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	if err := service.PublishPresenceDisconnected("presence.lifecycle", "user-auth"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var event PresenceLifecycleEvent
	if err := json.Unmarshal([]byte(repo.payload), &event); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}
	if event.Type != "disconnected" {
		t.Fatalf("expected disconnected event, got %s", event.Type)
	}
}

func TestPublishPresenceChatOpened(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	if err := service.PublishPresenceChatOpened("presence.lifecycle", "user-auth", "user-b"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var event PresenceLifecycleEvent
	if err := json.Unmarshal([]byte(repo.payload), &event); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}
	if event.Type != MessageTypeChatOpened {
		t.Fatalf("expected %s event, got %s", MessageTypeChatOpened, event.Type)
	}
	if event.CurrentChatID != "user-b" {
		t.Fatalf("expected current_chat_id user-b, got %s", event.CurrentChatID)
	}
}

func TestPublishPresenceChatClosed(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	if err := service.PublishPresenceChatClosed("presence.lifecycle", "user-auth"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var event PresenceLifecycleEvent
	if err := json.Unmarshal([]byte(repo.payload), &event); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}
	if event.Type != MessageTypeChatClosed {
		t.Fatalf("expected %s event, got %s", MessageTypeChatClosed, event.Type)
	}
}

func TestPublishChatInteraction(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	err := service.PublishChatInteraction("chat.events", "user-b", MessageTypeMessageSeen, ChatInteractionPayload{
		TargetUserID:  "user-a",
		CurrentChatID: "user-a",
		MessageID:     "mongo-id",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.channel != "chat.events" {
		t.Fatalf("expected channel chat.events, got %s", repo.channel)
	}

	var event ChatInteractionEvent
	if err := json.Unmarshal([]byte(repo.payload), &event); err != nil {
		t.Fatalf("expected valid JSON payload, got %v", err)
	}
	if event.Type != MessageTypeMessageSeen {
		t.Fatalf("expected %s event, got %s", MessageTypeMessageSeen, event.Type)
	}
	if event.ActorUserID != "user-b" {
		t.Fatalf("expected actor user-b, got %s", event.ActorUserID)
	}
	if event.TargetUserID != "user-a" {
		t.Fatalf("expected target user-a, got %s", event.TargetUserID)
	}
	if event.ViewedStatus != ViewedStatusSeen {
		t.Fatalf("expected viewed status %s, got %s", ViewedStatusSeen, event.ViewedStatus)
	}
}

func TestPublishChatInteractionRejectsInvalidPayload(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	err := service.PublishChatInteraction("chat.events", "user-b", MessageTypeMessageDelivered, ChatInteractionPayload{
		TargetUserID: "user-a",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPersistMessageReturnsRedisError(t *testing.T) {
	repo := &serviceRepositoryStub{addErr: ValidationError{Message: "redis unavailable"}}
	service := NewService(repo, "chat-stream")

	err := service.PersistMessage("user-auth", ChatMessagePayload{
		ReceiverID: "user-b",
		Content:    "hello",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "redis unavailable" {
		t.Fatalf("expected redis error, got %v", err)
	}
}

func TestPublishPresenceConnectedReturnsPublishError(t *testing.T) {
	repo := &serviceRepositoryStub{publishErr: ValidationError{Message: "publish failed"}}
	service := NewService(repo, "chat-stream")

	err := service.PublishPresenceConnected("presence.lifecycle", "user-auth")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "publish failed" {
		t.Fatalf("expected publish error, got %v", err)
	}
}

func TestPublishPresenceDisconnectedReturnsPublishError(t *testing.T) {
	repo := &serviceRepositoryStub{publishErr: ValidationError{Message: "publish failed"}}
	service := NewService(repo, "chat-stream")

	err := service.PublishPresenceDisconnected("presence.lifecycle", "user-auth")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "publish failed" {
		t.Fatalf("expected publish error, got %v", err)
	}
}

func TestPublishPresenceChatOpenedRejectsMissingCurrentChatID(t *testing.T) {
	repo := &serviceRepositoryStub{}
	service := NewService(repo, "chat-stream")

	if err := service.PublishPresenceChatOpened("presence.lifecycle", "user-auth", ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}
