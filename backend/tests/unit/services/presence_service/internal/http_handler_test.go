package presence_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	presence "github.com/Miguel-Pezzini/GoMessenger/services/presence_service/internal"
)

type repositoryStub struct {
	presence presence.Presence
	err      error
}

func (r *repositoryStub) Save(_ context.Context, _ presence.Presence) error {
	return nil
}

func (r *repositoryStub) Get(_ context.Context, _ string) (presence.Presence, error) {
	return r.presence, r.err
}

func (r *repositoryStub) Publish(_ context.Context, _ string, _ presence.Presence) error {
	return nil
}

func (r *repositoryStub) Subscribe(_ context.Context, _ string, _ func(presence.LifecycleEvent)) error {
	return nil
}

func TestHandleGetPresence(t *testing.T) {
	lastSeen := time.Date(2026, 4, 2, 20, 0, 0, 0, time.UTC)
	service := presence.NewService(&repositoryStub{
		presence: presence.Presence{
			UserID:        "user-a",
			Status:        presence.StatusOffline,
			LastSeen:      &lastSeen,
			CurrentChatID: "chat-1",
		},
	}, "presence.updated")
	handler := presence.NewHandler(service)
	mux := http.NewServeMux()
	mux.Handle("GET /presence/{userID}", http.HandlerFunc(handler.HandleGetPresence))

	req := httptest.NewRequest(http.MethodGet, "/presence/user-a", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	expected := "{\"user_id\":\"user-a\",\"status\":\"offline\",\"last_seen\":\"2026-04-02T20:00:00Z\",\"current_chat_id\":\"chat-1\"}\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, rec.Body.String())
	}
}

func TestHandleGetPresenceReturnsNotFound(t *testing.T) {
	service := presence.NewService(&repositoryStub{err: presence.ErrPresenceNotFound}, "presence.updated")
	handler := presence.NewHandler(service)
	mux := http.NewServeMux()
	mux.Handle("GET /presence/{userID}", http.HandlerFunc(handler.HandleGetPresence))

	req := httptest.NewRequest(http.MethodGet, "/presence/user-a", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
