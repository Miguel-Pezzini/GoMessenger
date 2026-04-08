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
