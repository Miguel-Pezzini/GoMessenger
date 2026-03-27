package domain

import "testing"

type repositoryStub struct {
	streamName string
	payload    string
	err        error
}

func (r *repositoryStub) AddToStream(streamName, payload string) error {
	r.streamName = streamName
	r.payload = payload
	return r.err
}

func (r *repositoryStub) Subscribe(_ string, _ func(payload string)) {}

func TestPersistMessageUsesAuthenticatedUserAsSender(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo, "chat-stream")

	err := service.PersistMessage("user-auth", ChatMessagePayload{
		ReceiverID: "user-b",
		Content:    "hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := `{"sender_id":"user-auth","receiver_id":"user-b","content":"hello"}`
	if repo.payload != expected {
		t.Fatalf("expected payload %s, got %s", expected, repo.payload)
	}
	if repo.streamName != "chat-stream" {
		t.Fatalf("expected stream chat-stream, got %s", repo.streamName)
	}
}

func TestPersistMessageRejectsMismatchedSenderID(t *testing.T) {
	repo := &repositoryStub{}
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
