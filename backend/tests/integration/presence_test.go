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

type presenceResponse struct {
	UserID        string     `json:"user_id"`
	Status        string     `json:"status"`
	LastSeen      *time.Time `json:"last_seen"`
	CurrentChatID string     `json:"current_chat_id"`
}

type activeUserResponse struct {
	UserID        string     `json:"user_id"`
	Username      string     `json:"username"`
	Status        string     `json:"status"`
	LastSeen      *time.Time `json:"last_seen"`
	CurrentChatID string     `json:"current_chat_id"`
}

type activeUsersResponse struct {
	Users []activeUserResponse `json:"users"`
	Count int                  `json:"count"`
}

func TestGetPresenceReturnsNotFoundForUnknownUser(t *testing.T) {
	t.Parallel()

	userID := fmt.Sprintf("missing-user-%d", time.Now().UnixNano())
	resp := getPresenceResponse(t, userID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestPresenceTracksWebsocketLifecycle(t *testing.T) {
	t.Parallel()

	username := fmt.Sprintf("presence_user_%d", time.Now().UnixNano())
	token := registerOrLogin(t, username, "123456")
	userID := extractUserIDFromJWT(t, token)

	conn := connectWS(t, token)

	onlinePresence := waitForPresenceStatus(t, userID, "online")
	if onlinePresence.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, onlinePresence.UserID)
	}
	if onlinePresence.LastSeen != nil {
		t.Fatalf("expected online presence to have nil last_seen, got %v", onlinePresence.LastSeen)
	}
	if onlinePresence.CurrentChatID != "" {
		t.Fatalf("expected empty current_chat_id, got %s", onlinePresence.CurrentChatID)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("failed to close websocket connection: %v", err)
	}

	offlinePresence := waitForPresenceStatus(t, userID, "offline")
	if offlinePresence.LastSeen == nil {
		t.Fatal("expected offline presence to include last_seen")
	}
	if offlinePresence.LastSeen.IsZero() {
		t.Fatal("expected offline presence last_seen to be set")
	}
	if offlinePresence.CurrentChatID != "" {
		t.Fatalf("expected empty current_chat_id, got %s", offlinePresence.CurrentChatID)
	}
}

func TestPresenceTracksCurrentChatLifecycle(t *testing.T) {
	t.Parallel()

	username1 := fmt.Sprintf("presence_chat_user_a_%d", time.Now().UnixNano())
	username2 := fmt.Sprintf("presence_chat_user_b_%d", time.Now().UnixNano())

	token1 := registerOrLogin(t, username1, "123456")
	token2 := registerOrLogin(t, username2, "123456")
	peerUserID := extractUserIDFromJWT(t, token2)
	userID := extractUserIDFromJWT(t, token1)

	conn := connectWS(t, token1)
	defer conn.Close()

	if err := conn.WriteJSON(gatewayMessage{
		Type: "chat_opened",
		Payload: map[string]string{
			"current_chat_id": peerUserID,
		},
	}); err != nil {
		t.Fatalf("failed to send chat_opened: %v", err)
	}

	openedPresence := waitForPresenceCurrentChat(t, userID, peerUserID)
	if openedPresence.Status != "online" {
		t.Fatalf("expected status online, got %s", openedPresence.Status)
	}

	if err := conn.WriteJSON(gatewayMessage{
		Type: "chat_closed",
		Payload: map[string]string{
			"current_chat_id": peerUserID,
		},
	}); err != nil {
		t.Fatalf("failed to send chat_closed: %v", err)
	}

	closedPresence := waitForPresenceCurrentChat(t, userID, "")
	if closedPresence.Status != "online" {
		t.Fatalf("expected status online, got %s", closedPresence.Status)
	}
}

func TestAdminActivePresenceRequiresAdminAndListsOnlineUsers(t *testing.T) {
	username := fmt.Sprintf("presence_admin_active_%d", time.Now().UnixNano())
	token := registerOrLogin(t, username, "123456")
	userID := extractUserIDFromJWT(t, token)
	conn := connectWS(t, token)
	defer conn.Close()

	_ = waitForPresenceStatus(t, userID, "online")

	req, err := http.NewRequest(http.MethodGet, presenceBaseURL+"/admin/presence/active?limit=100", nil)
	if err != nil {
		t.Fatalf("failed to create active presence request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+frontendToken(t))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("presence service unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to call active presence endpoint: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d for non-admin, got %d", http.StatusForbidden, resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodGet, presenceBaseURL+"/admin/presence/active?limit=100", nil)
	if err != nil {
		t.Fatalf("failed to create active presence request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("presence service unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to call active presence endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var active activeUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&active); err != nil {
		t.Fatalf("failed to decode active presence response: %v", err)
	}
	if active.Count != len(active.Users) {
		t.Fatalf("expected count to match users length, got count=%d len=%d", active.Count, len(active.Users))
	}
	for _, user := range active.Users {
		if user.UserID == userID {
			if user.Status != "online" {
				t.Fatalf("expected active user status online, got %s", user.Status)
			}
			if user.Username != username {
				t.Fatalf("expected username %s, got %s", username, user.Username)
			}
			return
		}
	}
	t.Fatalf("expected active response to include user %s, got %+v", userID, active.Users)
}

func TestAdminActivePresenceRemainsResponsiveWithMultipleOnlineUsers(t *testing.T) {
	const onlineUsers = 12

	var conns []interface{ Close() error }
	for i := 0; i < onlineUsers; i++ {
		username := fmt.Sprintf("presence_bulk_user_%d_%d", time.Now().UnixNano(), i)
		token := registerOrLogin(t, username, "123456")
		userID := extractUserIDFromJWT(t, token)
		conn := connectWS(t, token)
		conns = append(conns, conn)
		_ = waitForPresenceStatus(t, userID, "online")
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	req, err := http.NewRequest(http.MethodGet, presenceBaseURL+"/admin/presence/active?limit=100", nil)
	if err != nil {
		t.Fatalf("failed to create active presence request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken(t))

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("presence service unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to call active presence endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var active activeUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&active); err != nil {
		t.Fatalf("failed to decode active presence response: %v", err)
	}
	if active.Count < onlineUsers {
		t.Fatalf("expected at least %d active users, got %d", onlineUsers, active.Count)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("expected active presence to respond within 5s, got %v", elapsed)
	}
}

func waitForPresenceStatus(t *testing.T, userID, expectedStatus string) presenceResponse {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	var last presenceResponse

	for time.Now().Before(deadline) {
		resp := getPresenceResponse(t, userID)

		if resp.StatusCode == http.StatusOK {
			var presence presenceResponse
			if err := json.NewDecoder(resp.Body).Decode(&presence); err != nil {
				resp.Body.Close()
				t.Fatalf("failed to decode presence response: %v", err)
			}
			resp.Body.Close()

			last = presence
			if presence.Status == expectedStatus {
				return presence
			}
		} else {
			resp.Body.Close()
		}

		time.Sleep(150 * time.Millisecond)
	}

	t.Fatalf("presence for user %s did not reach status %s, last response: %+v", userID, expectedStatus, last)
	return presenceResponse{}
}

func waitForPresenceCurrentChat(t *testing.T, userID, expectedCurrentChatID string) presenceResponse {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	var last presenceResponse

	for time.Now().Before(deadline) {
		resp := getPresenceResponse(t, userID)

		if resp.StatusCode == http.StatusOK {
			var presence presenceResponse
			if err := json.NewDecoder(resp.Body).Decode(&presence); err != nil {
				resp.Body.Close()
				t.Fatalf("failed to decode presence response: %v", err)
			}
			resp.Body.Close()

			last = presence
			if presence.CurrentChatID == expectedCurrentChatID {
				return presence
			}
		} else {
			resp.Body.Close()
		}

		time.Sleep(150 * time.Millisecond)
	}

	t.Fatalf("presence for user %s did not reach current_chat_id %s, last response: %+v", userID, expectedCurrentChatID, last)
	return presenceResponse{}
}

func getPresenceResponse(t *testing.T, userID string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, presenceBaseURL+"/presence/"+userID, nil)
	if err != nil {
		t.Fatalf("failed to create presence request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+frontendToken(t))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("presence service unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to get presence for %s: %v", userID, err)
	}

	return resp
}
