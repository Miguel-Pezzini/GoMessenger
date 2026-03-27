package domain

import (
	"context"
	"errors"
	"testing"
)

type repositoryStub struct {
	message *MessageDB
	result  *MessageDB
	created bool
	err     error
}

func (r *repositoryStub) Create(_ context.Context, messageDB *MessageDB) (*MessageDB, bool, error) {
	r.message = messageDB
	if r.err != nil {
		return nil, false, r.err
	}
	return r.result, r.created, nil
}

func TestCreateUsesStreamIDAsIdempotencyKey(t *testing.T) {
	repo := &repositoryStub{
		result: &MessageDB{
			Id:         "mongo-id",
			StreamID:   "171234-0",
			SenderID:   "user-a",
			ReceiverID: "user-b",
			Content:    "hello",
			Timestamp:  123,
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
	if res.Id != "mongo-id" {
		t.Fatalf("expected response id mongo-id, got %s", res.Id)
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
