package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type auditLogEvent struct {
	StreamID     string         `json:"stream_id"`
	EventID      string         `json:"event_id"`
	EventType    string         `json:"event_type"`
	Category     string         `json:"category"`
	Service      string         `json:"service"`
	ActorUserID  string         `json:"actor_user_id"`
	TargetUserID string         `json:"target_user_id"`
	EntityType   string         `json:"entity_type"`
	EntityID     string         `json:"entity_id"`
	OccurredAt   string         `json:"occurred_at"`
	Status       string         `json:"status"`
	Message      string         `json:"message"`
	Metadata     map[string]any `json:"metadata"`
}

func TestLoggingEndpointsRequireAdminAuth(t *testing.T) {
	userToken := frontendToken(t)

	req, err := http.NewRequest(http.MethodGet, loggingBaseURL+"/logs", nil)
	if err != nil {
		t.Fatalf("failed to create logs request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("logging service unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to call logs endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+userToken)

	_, resp, err = websocket.DefaultDialer.Dial(websocketURL(loggingBaseURL, "/logs/ws"), headers)
	if err == nil {
		t.Fatal("expected websocket dial to fail without admin auth")
	}
	var netErr *net.OpError
	if resp == nil && errors.As(err, &netErr) {
		t.Skipf("logging service unavailable for integration test: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for unauthenticated websocket failure, got err %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestRegisterProducesLiveAndPersistedAuditLog(t *testing.T) {
	username := fmt.Sprintf("log_register_%d", time.Now().UnixNano())
	password := "123456"

	conn := connectAdminLogsWS(t)
	defer conn.Close()

	token := registerOrLogin(t, username, password)
	if token == "" {
		t.Fatal("expected token from register/login flow")
	}

	liveEvent := readLogEventUntil(t, conn, func(event auditLogEvent) bool {
		return event.EventType == "user.registered" && metadataString(event.Metadata, "username") == username
	})
	if liveEvent.Service != "auth" {
		t.Fatalf("expected auth service, got %s", liveEvent.Service)
	}

	storedEvent := waitForLogEvent(t, func(event auditLogEvent) bool {
		return event.EventType == "user.registered" && metadataString(event.Metadata, "username") == username
	})
	if storedEvent.Status != "success" {
		t.Fatalf("expected success status, got %s", storedEvent.Status)
	}
}

func TestImportantActionsProduceAuditHistory(t *testing.T) {
	password := "123456"
	usernameA := fmt.Sprintf("log_user_a_%d", time.Now().UnixNano())
	usernameB := fmt.Sprintf("log_user_b_%d", time.Now().UnixNano())

	tokenA := registerOrLogin(t, usernameA, password)
	tokenB := registerOrLogin(t, usernameB, password)
	userA := extractUserIDFromJWT(t, tokenA)
	userB := extractUserIDFromJWT(t, tokenB)

	failedLogin(t, usernameA, "wrong-password")
	waitForLogEvent(t, func(event auditLogEvent) bool {
		return event.EventType == "user.login.failed" && metadataString(event.Metadata, "username") == usernameA
	})

	request := sendFriendRequest(t, tokenA, userB)
	waitForLogEvent(t, func(event auditLogEvent) bool {
		return event.EventType == "friend_request.created" && event.ActorUserID == userA && event.TargetUserID == userB && event.EntityID == request.ID
	})

	connA := connectWS(t, tokenA)
	defer connA.Close()
	connB := connectWS(t, tokenB)
	defer connB.Close()

	waitForLogEvent(t, func(event auditLogEvent) bool {
		return event.EventType == "websocket.connected" && event.ActorUserID == userA
	})
	waitForLogEvent(t, func(event auditLogEvent) bool {
		return event.EventType == "websocket.connected" && event.ActorUserID == userB
	})

	message := gatewayMessage{
		Type: "chat_message",
		Payload: chatPayload{
			ReceiverID: userB,
			Content:    "logging integration",
		},
	}
	if err := connA.WriteJSON(message); err != nil {
		t.Fatalf("failed to send websocket message: %v", err)
	}

	_ = readMessageWithRetry(t, connA)
	_ = readMessageWithRetry(t, connB)

	waitForLogEvent(t, func(event auditLogEvent) bool {
		return event.EventType == "chat.message.persisted" && event.ActorUserID == userA && event.TargetUserID == userB
	})
}

func connectAdminLogsWS(t *testing.T) *websocket.Conn {
	t.Helper()

	token := adminToken(t)
	if role := extractRoleFromJWT(t, token); role != "ADMIN" {
		t.Fatalf("expected admin token role ADMIN, got %s", role)
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)

	conn, _, err := websocket.DefaultDialer.Dial(websocketURL(loggingBaseURL, "/logs/ws"), headers)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("logging websocket unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to connect logging websocket: %v", err)
	}
	return conn
}

func readLogEventUntil(t *testing.T, conn *websocket.Conn, match func(auditLogEvent) bool) auditLogEvent {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var event auditLogEvent
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatalf("failed to read log event: %v", err)
		}
		if match(event) {
			return event
		}
	}

	t.Fatal("expected matching log event before timeout")
	return auditLogEvent{}
}

func waitForLogEvent(t *testing.T, match func(auditLogEvent) bool) auditLogEvent {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		events := fetchLogs(t)
		for _, event := range events {
			if match(event) {
				return event
			}
		}
		time.Sleep(150 * time.Millisecond)
	}

	t.Fatal("expected matching log event before timeout")
	return auditLogEvent{}
}

func fetchLogs(t *testing.T) []auditLogEvent {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, loggingBaseURL+"/logs?limit=200", nil)
	if err != nil {
		t.Fatalf("failed to create log request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("logging service unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to fetch logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var events []auditLogEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("failed to decode logs response: %v", err)
	}
	return events
}

func failedLogin(t *testing.T, username, password string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		t.Fatalf("failed to marshal login body: %v", err)
	}

	resp, err := http.Post(gatewayBaseURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to call login endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}
