package chat

import (
	"context"
	"errors"
	"testing"
)

type repositoryStub struct {
	message          *MessageDB
	result           *MessageDB
	created          bool
	err              error
	conversationMsgs []MessageDB
	conversationErr  error
	updateMessageID  string
	updateReceiverID string
	updateStatus     string
	updateResult     *MessageDB
	updateErr        error
}

func (r *repositoryStub) Create(_ context.Context, messageDB *MessageDB) (*MessageDB, bool, error) {
	r.message = messageDB
	if r.err != nil {
		return nil, false, r.err
	}
	return r.result, r.created, nil
}

func (r *repositoryStub) GetConversation(_ context.Context, _, _, _ string, _ int) ([]MessageDB, error) {
	return r.conversationMsgs, r.conversationErr
}

func (r *repositoryStub) UpdateViewedStatus(_ context.Context, messageID, receiverUserID, status string) (*MessageDB, error) {
	r.updateMessageID = messageID
	r.updateReceiverID = receiverUserID
	r.updateStatus = status
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	return r.updateResult, nil
}

func TestCreateUsesStreamIDAsIdempotencyKey(t *testing.T) {
	repo := &repositoryStub{
		result: &MessageDB{
			Id:           "mongo-id",
			StreamID:     "171234-0",
			SenderID:     "user-a",
			ReceiverID:   "user-b",
			Content:      "hello",
			Timestamp:    123,
			ViewedStatus: ViewedStatusSent,
		},
	}
	service := NewService(repo)

	res, err := service.Create(context.Background(), MessageRequest{
		StreamID:   "171234-0",
		SenderID:   "user-a",
		ReceiverID: "user-b",
		Content:    "hello",
		Timestamp:  123,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.message == nil {
		t.Fatal("expected repository to receive a message")
	}
	if repo.message.StreamID != "171234-0" {
		t.Fatalf("expected stream id 171234-0, got %s", repo.message.StreamID)
	}
	if repo.message.ViewedStatus != ViewedStatusSent {
		t.Fatalf("expected viewed status %s, got %s", ViewedStatusSent, repo.message.ViewedStatus)
	}
	if res.Id != "mongo-id" {
		t.Fatalf("expected response id mongo-id, got %s", res.Id)
	}
	if res.ViewedStatus != ViewedStatusSent {
		t.Fatalf("expected response viewed status %s, got %s", ViewedStatusSent, res.ViewedStatus)
	}
}

func TestCreateReturnsRepositoryError(t *testing.T) {
	repo := &repositoryStub{err: errors.New("boom")}
	service := NewService(repo)

	_, err := service.Create(context.Background(), MessageRequest{StreamID: "1-0"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetConversationReturnsMessagesOldestFirst(t *testing.T) {
	// repo returns newest-first (as MongoDB does with sort -1)
	repo := &repositoryStub{
		conversationMsgs: []MessageDB{
			{Id: "c", SenderID: "user-a", ReceiverID: "user-b", Content: "third"},
			{Id: "b", SenderID: "user-b", ReceiverID: "user-a", Content: "second"},
			{Id: "a", SenderID: "user-a", ReceiverID: "user-b", Content: "first"},
		},
	}
	service := NewService(repo)

	result, err := service.GetConversation(context.Background(), "user-a", "user-b", "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HasMore {
		t.Fatal("expected has_more=false")
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
	// service reverses so oldest comes first
	if result.Messages[0].Id != "a" {
		t.Fatalf("expected first message id=a, got %s", result.Messages[0].Id)
	}
	if result.Messages[2].Id != "c" {
		t.Fatalf("expected last message id=c, got %s", result.Messages[2].Id)
	}
}

func TestGetConversationHasMoreWhenExtraReturned(t *testing.T) {
	// with limit=2 the service asks repo for limit+1=3; repo returns 3 → has_more=true
	repo := &repositoryStub{
		conversationMsgs: []MessageDB{
			{Id: "c", Content: "third"},
			{Id: "b", Content: "second"},
			{Id: "a", Content: "first"},
		},
	}
	service := NewService(repo)

	result, err := service.GetConversation(context.Background(), "user-a", "user-b", "", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.HasMore {
		t.Fatal("expected has_more=true")
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages (trimmed), got %d", len(result.Messages))
	}
}

func TestGetConversationUsesDefaultLimitForInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		limit int
	}{
		{"zero", 0},
		{"negative", -5},
		{"over max", 200},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &repositoryStub{conversationMsgs: []MessageDB{}}
			service := NewService(repo)
			_, err := service.GetConversation(context.Background(), "a", "b", "", tc.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetConversationPropagatesRepositoryError(t *testing.T) {
	repo := &repositoryStub{conversationErr: errors.New("db down")}
	service := NewService(repo)

	_, err := service.GetConversation(context.Background(), "a", "b", "", 20)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateViewedStatus(t *testing.T) {
	repo := &repositoryStub{
		updateResult: &MessageDB{
			Id:           "mongo-id",
			SenderID:     "user-a",
			ReceiverID:   "user-b",
			Content:      "hello",
			ViewedStatus: ViewedStatusSeen,
		},
	}
	service := NewService(repo)

	result, err := service.UpdateViewedStatus(context.Background(), "mongo-id", "user-b", ViewedStatusSeen)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.updateMessageID != "mongo-id" {
		t.Fatalf("expected message id mongo-id, got %s", repo.updateMessageID)
	}
	if repo.updateReceiverID != "user-b" {
		t.Fatalf("expected receiver user-b, got %s", repo.updateReceiverID)
	}
	if repo.updateStatus != ViewedStatusSeen {
		t.Fatalf("expected status %s, got %s", ViewedStatusSeen, repo.updateStatus)
	}
	if result.ViewedStatus != ViewedStatusSeen {
		t.Fatalf("expected response viewed status %s, got %s", ViewedStatusSeen, result.ViewedStatus)
	}
}

func TestUpdateViewedStatusRejectsSent(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo)

	if _, err := service.UpdateViewedStatus(context.Background(), "mongo-id", "user-b", ViewedStatusSent); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateViewedStatusPropagatesRepositoryError(t *testing.T) {
	repo := &repositoryStub{updateErr: errors.New("db down")}
	service := NewService(repo)

	if _, err := service.UpdateViewedStatus(context.Background(), "mongo-id", "user-b", ViewedStatusDelivered); err == nil {
		t.Fatal("expected error, got nil")
	}
}
