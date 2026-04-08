package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

type conversationResponse struct {
	Messages []wsMessageResponse `json:"messages"`
	HasMore  bool                `json:"has_more"`
}

// TestChatHistoryReturnsPersistedMessages sends N messages via WebSocket,
// then retrieves them through GET /messages/{userId} and asserts correctness.
func TestChatHistoryReturnsPersistedMessages(t *testing.T) {
	t.Parallel()

	ts := time.Now().UnixNano()
	userA := fmt.Sprintf("hist_a_%d", ts)
	userB := fmt.Sprintf("hist_b_%d", ts)

	tokenA := registerOrLogin(t, userA, "123456")
	tokenB := registerOrLogin(t, userB, "123456")

	idA := extractUserIDFromJWT(t, tokenA)
	idB := extractUserIDFromJWT(t, tokenB)

	// Connect both users over WebSocket so messages can be routed.
	connA := connectWS(t, tokenA)
	defer connA.Close()
	connB := connectWS(t, tokenB)
	defer connB.Close()

	// Send 3 messages from A → B.
	contents := []string{"first", "second", "third"}
	for _, c := range contents {
		msg := gatewayMessage{
			Type: "chat_message",
			Payload: chatPayload{
				ReceiverID: idB,
				Content:    c,
			},
		}
		if err := connA.WriteJSON(msg); err != nil {
			t.Fatalf("failed to send message %q: %v", c, err)
		}
		// drain sender echo so buffers don't fill
		readMessageWithRetry(t, connA)
		readMessageWithRetry(t, connB)
	}

	// Poll until chat service has persisted all 3 messages (stream → mongo is async).
	var result conversationResponse
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r, err := getConversation(t, tokenA, idB, "", 0)
		if err != nil {
			t.Fatalf("unexpected HTTP error: %v", err)
		}
		if len(r.Messages) >= len(contents) {
			result = r
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if len(result.Messages) < len(contents) {
		t.Fatalf("expected %d messages in history, got %d", len(contents), len(result.Messages))
	}

	// Messages must be ordered oldest → newest.
	for i, want := range contents {
		got := result.Messages[i]
		if got.Content != want {
			t.Fatalf("message[%d]: expected content %q, got %q", i, want, got.Content)
		}
		if got.SenderID != idA {
			t.Fatalf("message[%d]: expected sender %s, got %s", i, idA, got.SenderID)
		}
		if got.ReceiverID != idB {
			t.Fatalf("message[%d]: expected receiver %s, got %s", i, idB, got.ReceiverID)
		}
		if got.ID == "" {
			t.Fatalf("message[%d]: expected non-empty id", i)
		}
		if got.ViewedStatus != "sent" {
			t.Fatalf("message[%d]: expected viewed_status sent, got %s", i, got.ViewedStatus)
		}
	}
}

// TestChatHistoryCursorPagination sends 3 messages and verifies that
// requesting before=<second_message_id> returns only the first message.
func TestChatHistoryCursorPagination(t *testing.T) {
	t.Parallel()

	ts := time.Now().UnixNano()
	userA := fmt.Sprintf("cursor_a_%d", ts)
	userB := fmt.Sprintf("cursor_b_%d", ts)

	tokenA := registerOrLogin(t, userA, "123456")
	tokenB := registerOrLogin(t, userB, "123456")
	idB := extractUserIDFromJWT(t, tokenB)

	connA := connectWS(t, tokenA)
	defer connA.Close()
	connB := connectWS(t, tokenB)
	defer connB.Close()

	for _, c := range []string{"page-first", "page-second", "page-third"} {
		msg := gatewayMessage{
			Type: "chat_message",
			Payload: chatPayload{
				ReceiverID: idB,
				Content:    c,
			},
		}
		if err := connA.WriteJSON(msg); err != nil {
			t.Fatalf("failed to send message: %v", err)
		}
		readMessageWithRetry(t, connA)
		readMessageWithRetry(t, connB)
	}

	// Wait for all 3 to be persisted.
	var all conversationResponse
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r, err := getConversation(t, tokenA, idB, "", 0)
		if err != nil {
			t.Fatalf("unexpected HTTP error: %v", err)
		}
		if len(r.Messages) >= 3 {
			all = r
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(all.Messages) < 3 {
		t.Fatalf("expected 3 persisted messages, got %d", len(all.Messages))
	}

	// Use the ID of the second message as the cursor.
	// Result should contain only the first (oldest) message.
	cursorID := all.Messages[1].ID
	page, err := getConversation(t, tokenA, idB, cursorID, 0)
	if err != nil {
		t.Fatalf("unexpected HTTP error on cursor query: %v", err)
	}

	if len(page.Messages) != 1 {
		t.Fatalf("expected 1 message before cursor, got %d", len(page.Messages))
	}
	if page.Messages[0].Content != "page-first" {
		t.Fatalf("expected content %q, got %q", "page-first", page.Messages[0].Content)
	}
	if page.HasMore {
		t.Fatal("expected has_more=false when only 1 message precedes cursor")
	}
}

// TestChatHistoryLimitParam verifies the limit query param is respected.
func TestChatHistoryLimitParam(t *testing.T) {
	t.Parallel()

	ts := time.Now().UnixNano()
	userA := fmt.Sprintf("limit_a_%d", ts)
	userB := fmt.Sprintf("limit_b_%d", ts)

	tokenA := registerOrLogin(t, userA, "123456")
	tokenB := registerOrLogin(t, userB, "123456")
	idB := extractUserIDFromJWT(t, tokenB)

	connA := connectWS(t, tokenA)
	defer connA.Close()
	connB := connectWS(t, tokenB)
	defer connB.Close()

	for _, c := range []string{"lim-1", "lim-2", "lim-3"} {
		msg := gatewayMessage{
			Type: "chat_message",
			Payload: chatPayload{
				ReceiverID: idB,
				Content:    c,
			},
		}
		if err := connA.WriteJSON(msg); err != nil {
			t.Fatalf("failed to send message: %v", err)
		}
		readMessageWithRetry(t, connA)
		readMessageWithRetry(t, connB)
	}

	// Wait for persistence.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r, err := getConversation(t, tokenA, idB, "", 0)
		if err != nil {
			t.Fatalf("unexpected HTTP error: %v", err)
		}
		if len(r.Messages) >= 3 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Request only 2 messages — should get the 2 newest and has_more=true.
	result, err := getConversation(t, tokenA, idB, "", 2)
	if err != nil {
		t.Fatalf("unexpected HTTP error: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages with limit=2, got %d", len(result.Messages))
	}
	if !result.HasMore {
		t.Fatal("expected has_more=true when more messages exist beyond limit")
	}
}

// TestChatHistoryRequiresAuth verifies that the endpoint rejects unauthenticated requests.
func TestChatHistoryRequiresAuth(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, gatewayBaseURL+"/messages/some-user-id", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for integration test: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestChatHistoryEmptyConversation returns an empty list when no messages have been sent.
func TestChatHistoryEmptyConversation(t *testing.T) {
	t.Parallel()

	ts := time.Now().UnixNano()
	tokenA := registerOrLogin(t, fmt.Sprintf("empty_a_%d", ts), "123456")
	tokenB := registerOrLogin(t, fmt.Sprintf("empty_b_%d", ts), "123456")
	idB := extractUserIDFromJWT(t, tokenB)

	result, err := getConversation(t, tokenA, idB, "", 0)
	if err != nil {
		t.Fatalf("unexpected HTTP error: %v", err)
	}

	if len(result.Messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(result.Messages))
	}
	if result.HasMore {
		t.Fatal("expected has_more=false for empty conversation")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func getConversation(t *testing.T, token, otherUserID, before string, limit int) (conversationResponse, error) {
	t.Helper()

	u := fmt.Sprintf("%s/messages/%s", gatewayBaseURL, otherUserID)
	if before != "" || limit > 0 {
		u += "?"
		if before != "" {
			u += "before=" + before
		}
		if limit > 0 {
			if before != "" {
				u += "&"
			}
			u += fmt.Sprintf("limit=%d", limit)
		}
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return conversationResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for integration test: %v", err)
		}
		return conversationResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return conversationResponse{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result conversationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return conversationResponse{}, fmt.Errorf("decode error: %w", err)
	}
	return result, nil
}
