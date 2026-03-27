package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type gatewayMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type chatPayload struct {
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
}

type wsMessageResponse struct {
	ID         string `json:"id"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp"`
}

func TestWebsocketTwoUsersExchangeMessage(t *testing.T) {
	t.Parallel()

	timestamp := time.Now().UnixNano()
	username1 := fmt.Sprintf("ws_user_a_%d", timestamp)
	username2 := fmt.Sprintf("ws_user_b_%d", timestamp)
	password := "123456"

	token1 := registerOrLogin(t, username1, password)
	token2 := registerOrLogin(t, username2, password)

	senderID := extractUserIDFromJWT(t, token1)
	receiverID := extractUserIDFromJWT(t, token2)

	conn1 := connectWS(t, token1)
	defer conn1.Close()
	conn2 := connectWS(t, token2)
	defer conn2.Close()

	expected := chatPayload{
		ReceiverID: receiverID,
		Content:    "olaa do websocket",
	}

	msg := gatewayMessage{
		Type:    "chat_message",
		Payload: expected,
	}

	if err := conn1.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send ws message: %v", err)
	}

	expected.SenderID = senderID

	gotFromSender := readMessageWithRetry(t, conn1)
	gotFromReceiver := readMessageWithRetry(t, conn2)

	assertChatMessage(t, gotFromSender, expected)
	assertChatMessage(t, gotFromReceiver, expected)
}

func registerOrLogin(t *testing.T, username, password string) string {
	t.Helper()

	requestBody := map[string]string{"username": username, "password": password}
	body, _ := json.Marshal(requestBody)

	registerResp, err := http.Post("http://localhost:8080/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for e2e test: %v", err)
		}
		t.Fatalf("failed to call register endpoint: %v", err)
	}
	defer registerResp.Body.Close()

	if registerResp.StatusCode == http.StatusCreated {
		var registerData RegisterResponse
		if err := json.NewDecoder(registerResp.Body).Decode(&registerData); err != nil {
			t.Fatalf("failed to decode register response: %v", err)
		}
		if registerData.Token == "" {
			t.Fatalf("register returned empty token")
		}
		return registerData.Token
	}

	loginResp, err := http.Post("http://localhost:8080/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to call login endpoint: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected login status code %d", loginResp.StatusCode)
	}

	var loginData RegisterResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginData); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginData.Token == "" {
		t.Fatalf("login returned empty token")
	}
	return loginData.Token
}

func connectWS(t *testing.T, token string) *websocket.Conn {
	t.Helper()

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws", RawQuery: "token=" + url.QueryEscape(token)}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	return conn
}

func readMessageWithRetry(t *testing.T, conn *websocket.Conn) wsMessageResponse {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))

	for {
		var msg wsMessageResponse
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("failed reading websocket response: %v", err)
		}
		if msg.Content != "" {
			return msg
		}
	}
}

func assertChatMessage(t *testing.T, got wsMessageResponse, expected chatPayload) {
	t.Helper()

	if got.SenderID != expected.SenderID {
		t.Fatalf("sender mismatch: expected %s got %s", expected.SenderID, got.SenderID)
	}
	if got.ReceiverID != expected.ReceiverID {
		t.Fatalf("receiver mismatch: expected %s got %s", expected.ReceiverID, got.ReceiverID)
	}
	if got.Content != expected.Content {
		t.Fatalf("content mismatch: expected %s got %s", expected.Content, got.Content)
	}
	if strings.TrimSpace(got.ID) == "" {
		t.Fatalf("expected persisted message id, got empty")
	}
}

func extractUserIDFromJWT(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode jwt payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("failed to unmarshal jwt payload: %v", err)
	}

	id, _ := claims["userId"].(string)
	if id == "" {
		t.Fatalf("JWT missing userId claim")
	}
	return id
}
