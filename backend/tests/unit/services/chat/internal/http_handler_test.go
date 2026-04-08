package chat_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	chat "github.com/Miguel-Pezzini/GoMessenger/services/chat/internal"
)

// repoStub satisfies chat.Repository for unit tests.
type repoStub struct {
	msgs []chat.MessageDB
	err  error
}

func (r *repoStub) Create(_ context.Context, _ *chat.MessageDB) (*chat.MessageDB, bool, error) {
	return nil, false, nil
}

func (r *repoStub) GetConversation(_ context.Context, _, _, _ string, _ int) ([]chat.MessageDB, error) {
	return r.msgs, r.err
}

func (r *repoStub) UpdateViewedStatus(_ context.Context, _, _, _ string) (*chat.MessageDB, error) {
	return nil, nil
}

func newRequest(method, target, userID string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	return req
}

func TestGetConversationUnauthorizedWithoutHeader(t *testing.T) {
	svc := chat.NewService(&repoStub{})
	h := chat.NewHandler(svc)

	req := newRequest(http.MethodGet, "/messages/user-b", "")
	// inject path value manually
	req.SetPathValue("userId", "user-b")
	w := httptest.NewRecorder()

	h.GetConversation(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetConversationReturnsBadRequestForInvalidLimit(t *testing.T) {
	svc := chat.NewService(&repoStub{})
	h := chat.NewHandler(svc)

	req := newRequest(http.MethodGet, "/messages/user-b?limit=abc", "user-a")
	req.SetPathValue("userId", "user-b")
	w := httptest.NewRecorder()

	h.GetConversation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetConversationReturnsInternalErrorOnRepoFailure(t *testing.T) {
	svc := chat.NewService(&repoStub{err: errors.New("mongo down")})
	h := chat.NewHandler(svc)

	req := newRequest(http.MethodGet, "/messages/user-b", "user-a")
	req.SetPathValue("userId", "user-b")
	w := httptest.NewRecorder()

	h.GetConversation(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetConversationReturnsOKWithMessages(t *testing.T) {
	msgs := []chat.MessageDB{
		{Id: "b", SenderID: "user-a", ReceiverID: "user-b", Content: "second"},
		{Id: "a", SenderID: "user-a", ReceiverID: "user-b", Content: "first"},
	}
	svc := chat.NewService(&repoStub{msgs: msgs})
	h := chat.NewHandler(svc)

	req := newRequest(http.MethodGet, "/messages/user-b", "user-a")
	req.SetPathValue("userId", "user-b")
	w := httptest.NewRecorder()

	h.GetConversation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp chat.ConversationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.Messages))
	}
	// service reverses to oldest-first; "a" is the oldest after reversing "b","a"
	if resp.Messages[0].Id != "a" {
		t.Fatalf("expected first message id=a, got %s", resp.Messages[0].Id)
	}
}

func TestGetConversationHasMoreFlag(t *testing.T) {
	// 21 messages returned from repo with default limit=20 → has_more=true, 20 in response
	msgs := make([]chat.MessageDB, 21)
	for i := range msgs {
		msgs[i] = chat.MessageDB{Id: string(rune('a' + i)), Content: "msg"}
	}
	svc := chat.NewService(&repoStub{msgs: msgs})
	h := chat.NewHandler(svc)

	req := newRequest(http.MethodGet, "/messages/user-b", "user-a")
	req.SetPathValue("userId", "user-b")
	w := httptest.NewRecorder()

	h.GetConversation(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp chat.ConversationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
	if len(resp.Messages) != 20 {
		t.Fatalf("expected 20 messages, got %d", len(resp.Messages))
	}
}
