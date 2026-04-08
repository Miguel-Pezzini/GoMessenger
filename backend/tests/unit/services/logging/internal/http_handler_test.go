package logging_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/security"
	logsvc "github.com/Miguel-Pezzini/GoMessenger/services/logging/internal"
	"github.com/gorilla/websocket"
)

type repositoryStub struct {
	events []logsvc.StoredEvent
}

func (r *repositoryStub) Append(_ context.Context, event logsvc.StoredEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *repositoryStub) ListRecent(_ context.Context, limit int) ([]logsvc.StoredEvent, error) {
	if limit > len(r.events) {
		limit = len(r.events)
	}
	return append([]logsvc.StoredEvent(nil), r.events[:limit]...), nil
}

func TestListLogsReturnsOKWithoutDirectAuthCheck(t *testing.T) {
	handler := logsvc.NewHandler(logsvc.NewService(&repositoryStub{}), security.NewOriginValidator(nil))
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	handler.ListLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestListLogsReturnsStoredEvents(t *testing.T) {
	repo := &repositoryStub{
		events: []logsvc.StoredEvent{{
			StreamID: "1-0",
			Event: audit.Event{
				EventID:    "evt-1",
				EventType:  "user.registered",
				Category:   audit.CategoryAudit,
				Service:    "auth",
				Status:     audit.StatusSuccess,
				Message:    "user registered",
				OccurredAt: time.Now().UTC().Format(time.RFC3339),
			},
		}},
	}
	handler := logsvc.NewHandler(logsvc.NewService(repo), security.NewOriginValidator(nil))
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=1", nil)
	rec := httptest.NewRecorder()

	handler.ListLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var events []logsvc.StoredEvent
	if err := json.NewDecoder(rec.Body).Decode(&events); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
}

func TestStreamLogsBroadcastsEventsToAuthorizedAdmin(t *testing.T) {
	service := logsvc.NewService(&repositoryStub{})
	handler := logsvc.NewHandler(service, security.NewOriginValidator(nil))
	server := httptest.NewServer(http.HandlerFunc(handler.StreamLogs))
	defer server.Close()

	wsURL := websocketURL(t, server.URL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer conn.Close()

	_, err = service.Ingest(context.Background(), "1-0", audit.Event{
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

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got logsvc.StoredEvent
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("failed to read websocket event: %v", err)
	}
	if got.EventType != "user.registered" {
		t.Fatalf("expected event type user.registered, got %s", got.EventType)
	}
}

func TestStreamLogsRejectsDisallowedOrigin(t *testing.T) {
	service := logsvc.NewService(&repositoryStub{})
	handler := logsvc.NewHandler(service, security.NewOriginValidator([]string{"http://allowed.test"}))
	server := httptest.NewServer(http.HandlerFunc(handler.StreamLogs))
	defer server.Close()

	wsURL := websocketURL(t, server.URL)
	headers := http.Header{}
	headers.Set("Origin", "http://blocked.test")

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		t.Fatal("expected websocket dial to fail")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response, got err %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func websocketURL(t *testing.T, serverURL string) string {
	t.Helper()

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse server url: %v", err)
	}
	parsed.Scheme = "ws"
	return parsed.String()
}
