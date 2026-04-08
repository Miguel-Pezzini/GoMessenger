package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var (
	frontendReaderTokenOnce sync.Once
	frontendReaderToken     string
	frontendReaderTokenErr  error
	adminReaderTokenOnce    sync.Once
	adminReaderToken        string
	adminReaderTokenErr     error
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
	ID           string `json:"id"`
	SenderID     string `json:"sender_id"`
	ReceiverID   string `json:"receiver_id"`
	Content      string `json:"content"`
	Timestamp    int64  `json:"timestamp"`
	ViewedStatus string `json:"viewed_status"`
}

type wsRealtimeEvent struct {
	Type    string                 `json:"type"`
	Payload wsRealtimeEventPayload `json:"payload"`
}

type wsRealtimeEventPayload struct {
	ActorUserID   string `json:"actor_user_id"`
	CurrentChatID string `json:"current_chat_id"`
	MessageID     string `json:"message_id"`
	ViewedStatus  string `json:"viewed_status"`
	OccurredAt    string `json:"occurred_at"`
}

type wsErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type wsNotificationMessage struct {
	Type    string                `json:"type"`
	Payload wsNotificationPayload `json:"payload"`
}

type wsNotificationPayload struct {
	NotificationType string `json:"notification_type"`
	RecipientUserID  string `json:"recipient_user_id"`
	ActorUserID      string `json:"actor_user_id"`
	EntityID         string `json:"entity_id"`
	ConversationID   string `json:"conversation_id"`
	Preview          string `json:"preview"`
	OccurredAt       string `json:"occurred_at"`
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
	if gotFromSender.ViewedStatus != "sent" {
		t.Fatalf("expected sender viewed status sent, got %s", gotFromSender.ViewedStatus)
	}
	if gotFromReceiver.ViewedStatus != "sent" {
		t.Fatalf("expected receiver viewed status sent, got %s", gotFromReceiver.ViewedStatus)
	}
}

func TestWebsocketConcurrentFanOutAcrossIndependentUsers(t *testing.T) {
	t.Parallel()

	timestamp := time.Now().UnixNano()
	password := "123456"

	tokenA := registerOrLogin(t, fmt.Sprintf("ws_fanout_a_%d", timestamp), password)
	tokenB := registerOrLogin(t, fmt.Sprintf("ws_fanout_b_%d", timestamp), password)
	tokenC := registerOrLogin(t, fmt.Sprintf("ws_fanout_c_%d", timestamp), password)
	tokenD := registerOrLogin(t, fmt.Sprintf("ws_fanout_d_%d", timestamp), password)

	senderA := extractUserIDFromJWT(t, tokenA)
	receiverB := extractUserIDFromJWT(t, tokenB)
	senderC := extractUserIDFromJWT(t, tokenC)
	receiverD := extractUserIDFromJWT(t, tokenD)

	connA := connectWS(t, tokenA)
	defer connA.Close()
	connB := connectWS(t, tokenB)
	defer connB.Close()
	connC := connectWS(t, tokenC)
	defer connC.Close()
	connD := connectWS(t, tokenD)
	defer connD.Close()

	errs := make(chan error, 2)
	go func() {
		errs <- connA.WriteJSON(gatewayMessage{
			Type: "chat_message",
			Payload: chatPayload{
				ReceiverID: receiverB,
				Content:    "fanout message ab",
			},
		})
	}()
	go func() {
		errs <- connC.WriteJSON(gatewayMessage{
			Type: "chat_message",
			Payload: chatPayload{
				ReceiverID: receiverD,
				Content:    "fanout message cd",
			},
		})
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("failed to send websocket message: %v", err)
		}
	}

	assertChatMessage(t, readMessageWithRetry(t, connA), chatPayload{
		SenderID:   senderA,
		ReceiverID: receiverB,
		Content:    "fanout message ab",
	})
	assertChatMessage(t, readMessageWithRetry(t, connB), chatPayload{
		SenderID:   senderA,
		ReceiverID: receiverB,
		Content:    "fanout message ab",
	})
	assertChatMessage(t, readMessageWithRetry(t, connC), chatPayload{
		SenderID:   senderC,
		ReceiverID: receiverD,
		Content:    "fanout message cd",
	})
	assertChatMessage(t, readMessageWithRetry(t, connD), chatPayload{
		SenderID:   senderC,
		ReceiverID: receiverD,
		Content:    "fanout message cd",
	})
}

func TestWebsocketRejectsUnauthenticatedConnection(t *testing.T) {
	t.Parallel()

	_, resp, err := websocket.DefaultDialer.Dial(websocketURL(gatewayBaseURL, "/ws"), nil)
	if err == nil {
		t.Fatal("expected websocket dial to fail without token")
	}
	var netErr *net.OpError
	if resp == nil && errors.As(err, &netErr) {
		t.Skipf("gateway unavailable for integration test: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for unauthenticated websocket failure, got err %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestWebsocketRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	headers := http.Header{}
	headers.Set("Authorization", "Bearer invalid.token.value")

	_, resp, err := websocket.DefaultDialer.Dial(websocketURL(gatewayBaseURL, "/ws"), headers)
	if err == nil {
		t.Fatal("expected websocket dial to fail with invalid token")
	}
	var netErr *net.OpError
	if resp == nil && errors.As(err, &netErr) {
		t.Skipf("gateway unavailable for integration test: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for invalid token websocket failure, got err %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestWebsocketRejectsDisallowedOrigin(t *testing.T) {
	t.Parallel()

	username := fmt.Sprintf("ws_origin_%d", time.Now().UnixNano())
	token := registerOrLogin(t, username, "123456")

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("Origin", "http://blocked.test")

	_, resp, err := websocket.DefaultDialer.Dial(websocketURL(gatewayBaseURL, "/ws"), headers)
	if err == nil {
		t.Fatal("expected websocket dial to fail with blocked origin")
	}
	var netErr *net.OpError
	if resp == nil && errors.As(err, &netErr) {
		t.Skipf("gateway unavailable for integration test: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for blocked origin, got err %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestWebsocketClientCanReconnect(t *testing.T) {
	t.Parallel()

	username := fmt.Sprintf("ws_reconnect_%d", time.Now().UnixNano())
	token := registerOrLogin(t, username, "123456")

	firstConn := connectWS(t, token)
	if err := firstConn.Close(); err != nil {
		t.Fatalf("failed to close first websocket connection: %v", err)
	}

	secondConn := connectWS(t, token)
	if err := secondConn.Close(); err != nil {
		t.Fatalf("failed to close second websocket connection: %v", err)
	}
}

func TestWebsocketReturnsValidationErrorForInvalidChatPayload(t *testing.T) {
	t.Parallel()

	username := fmt.Sprintf("ws_invalid_payload_%d", time.Now().UnixNano())
	token := registerOrLogin(t, username, "123456")
	conn := connectWS(t, token)
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"type":    "chat_message",
		"payload": map[string]any{"content": "hello"},
	}); err != nil {
		t.Fatalf("failed to send ws message: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	var got wsErrorResponse
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("failed reading websocket response: %v", err)
	}
	if got.Type != "error" {
		t.Fatalf("expected error message type, got %s", got.Type)
	}
	if got.Error.Message != "receiver_id is required" {
		t.Fatalf("expected receiver_id validation error, got %s", got.Error.Message)
	}
}

func TestTypingIndicatorFanOut(t *testing.T) {
	t.Parallel()

	timestamp := time.Now().UnixNano()
	username1 := fmt.Sprintf("typing_user_a_%d", timestamp)
	username2 := fmt.Sprintf("typing_user_b_%d", timestamp)

	token1 := registerOrLogin(t, username1, "123456")
	token2 := registerOrLogin(t, username2, "123456")

	senderID := extractUserIDFromJWT(t, token1)
	receiverID := extractUserIDFromJWT(t, token2)

	conn1 := connectWS(t, token1)
	defer conn1.Close()
	conn2 := connectWS(t, token2)
	defer conn2.Close()

	if err := conn1.WriteJSON(gatewayMessage{
		Type: "typing_started",
		Payload: map[string]string{
			"target_user_id":  receiverID,
			"current_chat_id": receiverID,
		},
	}); err != nil {
		t.Fatalf("failed to publish typing_started: %v", err)
	}

	started := readRealtimeEventWithRetry(t, conn2)
	if started.Type != "typing_started" {
		t.Fatalf("expected typing_started, got %s", started.Type)
	}
	if started.Payload.ActorUserID != senderID {
		t.Fatalf("expected actor %s, got %s", senderID, started.Payload.ActorUserID)
	}
	if started.Payload.CurrentChatID != receiverID {
		t.Fatalf("expected current_chat_id %s, got %s", receiverID, started.Payload.CurrentChatID)
	}

	if err := conn1.WriteJSON(gatewayMessage{
		Type: "typing_stopped",
		Payload: map[string]string{
			"target_user_id":  receiverID,
			"current_chat_id": receiverID,
		},
	}); err != nil {
		t.Fatalf("failed to publish typing_stopped: %v", err)
	}

	stopped := readRealtimeEventWithRetry(t, conn2)
	if stopped.Type != "typing_stopped" {
		t.Fatalf("expected typing_stopped, got %s", stopped.Type)
	}
	if stopped.Payload.ActorUserID != senderID {
		t.Fatalf("expected actor %s, got %s", senderID, stopped.Payload.ActorUserID)
	}
}

func TestViewedStatusLifecycle(t *testing.T) {
	t.Parallel()

	timestamp := time.Now().UnixNano()
	username1 := fmt.Sprintf("status_user_a_%d", timestamp)
	username2 := fmt.Sprintf("status_user_b_%d", timestamp)

	token1 := registerOrLogin(t, username1, "123456")
	token2 := registerOrLogin(t, username2, "123456")

	senderID := extractUserIDFromJWT(t, token1)
	receiverID := extractUserIDFromJWT(t, token2)

	conn1 := connectWS(t, token1)
	defer conn1.Close()
	conn2 := connectWS(t, token2)
	defer conn2.Close()

	if err := conn1.WriteJSON(gatewayMessage{
		Type: "chat_message",
		Payload: chatPayload{
			ReceiverID: receiverID,
			Content:    "status test",
		},
	}); err != nil {
		t.Fatalf("failed to send chat message: %v", err)
	}

	fromSender := readMessageWithRetry(t, conn1)
	fromReceiver := readMessageWithRetry(t, conn2)
	if fromSender.ID == "" {
		t.Fatal("expected sender message id")
	}
	if fromSender.ViewedStatus != "sent" {
		t.Fatalf("expected sender viewed status sent, got %s", fromSender.ViewedStatus)
	}
	if fromReceiver.ViewedStatus != "sent" {
		t.Fatalf("expected receiver viewed status sent, got %s", fromReceiver.ViewedStatus)
	}

	if err := conn2.WriteJSON(gatewayMessage{
		Type: "message_delivered",
		Payload: map[string]string{
			"target_user_id":  senderID,
			"current_chat_id": senderID,
			"message_id":      fromReceiver.ID,
		},
	}); err != nil {
		t.Fatalf("failed to publish message_delivered: %v", err)
	}

	delivered := readRealtimeEventWithRetry(t, conn1)
	if delivered.Type != "message_delivered" {
		t.Fatalf("expected message_delivered, got %s", delivered.Type)
	}
	if delivered.Payload.MessageID != fromReceiver.ID {
		t.Fatalf("expected message id %s, got %s", fromReceiver.ID, delivered.Payload.MessageID)
	}
	if delivered.Payload.ViewedStatus != "delivered" {
		t.Fatalf("expected viewed status delivered, got %s", delivered.Payload.ViewedStatus)
	}

	waitForViewedStatus(t, token1, receiverID, fromReceiver.ID, "delivered")

	if err := conn2.WriteJSON(gatewayMessage{
		Type: "message_seen",
		Payload: map[string]string{
			"target_user_id":  senderID,
			"current_chat_id": senderID,
			"message_id":      fromReceiver.ID,
		},
	}); err != nil {
		t.Fatalf("failed to publish message_seen: %v", err)
	}

	seen := readRealtimeEventWithRetry(t, conn1)
	if seen.Type != "message_seen" {
		t.Fatalf("expected message_seen, got %s", seen.Type)
	}
	if seen.Payload.MessageID != fromReceiver.ID {
		t.Fatalf("expected message id %s, got %s", fromReceiver.ID, seen.Payload.MessageID)
	}
	if seen.Payload.ViewedStatus != "seen" {
		t.Fatalf("expected viewed status seen, got %s", seen.Payload.ViewedStatus)
	}

	waitForViewedStatus(t, token1, receiverID, fromReceiver.ID, "seen")
}

func TestFriendRequestNotificationDeliveredOverWebsocket(t *testing.T) {
	t.Parallel()

	senderToken, receiverToken, senderID, receiverID := newFriendTestUsers(t, "friend_notification")

	conn := connectWS(t, receiverToken)
	defer conn.Close()

	request := sendFriendRequest(t, senderToken, receiverID)
	notification := readNotificationWithRetry(t, conn)

	if notification.Type != "notification" {
		t.Fatalf("expected notification type, got %s", notification.Type)
	}
	if notification.Payload.NotificationType != "friend_request_received" {
		t.Fatalf("expected friend_request_received, got %s", notification.Payload.NotificationType)
	}
	if notification.Payload.RecipientUserID != receiverID {
		t.Fatalf("expected recipient %s, got %s", receiverID, notification.Payload.RecipientUserID)
	}
	if notification.Payload.ActorUserID != senderID {
		t.Fatalf("expected actor %s, got %s", senderID, notification.Payload.ActorUserID)
	}
	if notification.Payload.EntityID != request.ID {
		t.Fatalf("expected request id %s, got %s", request.ID, notification.Payload.EntityID)
	}
}

func TestMessageNotificationDeliveredWhenReceiverInDifferentChat(t *testing.T) {
	t.Parallel()

	timestamp := time.Now().UnixNano()
	tokenSender := registerOrLogin(t, fmt.Sprintf("notify_sender_%d", timestamp), "123456")
	tokenReceiver := registerOrLogin(t, fmt.Sprintf("notify_receiver_%d", timestamp), "123456")
	tokenOther := registerOrLogin(t, fmt.Sprintf("notify_other_%d", timestamp), "123456")

	senderID := extractUserIDFromJWT(t, tokenSender)
	receiverID := extractUserIDFromJWT(t, tokenReceiver)
	otherID := extractUserIDFromJWT(t, tokenOther)

	connSender := connectWS(t, tokenSender)
	defer connSender.Close()
	connReceiver := connectWS(t, tokenReceiver)
	defer connReceiver.Close()

	if err := connReceiver.WriteJSON(gatewayMessage{
		Type: "chat_opened",
		Payload: map[string]string{
			"current_chat_id": otherID,
		},
	}); err != nil {
		t.Fatalf("failed to open other chat: %v", err)
	}
	waitForPresenceCurrentChat(t, receiverID, otherID)

	if err := connSender.WriteJSON(gatewayMessage{
		Type: "chat_message",
		Payload: chatPayload{
			ReceiverID: receiverID,
			Content:    "notification hello",
		},
	}); err != nil {
		t.Fatalf("failed to send chat message: %v", err)
	}

	_ = readMessageWithRetry(t, connSender)
	_ = readMessageWithRetry(t, connReceiver)
	notification := readNotificationWithRetry(t, connReceiver)

	if notification.Payload.NotificationType != "message_received" {
		t.Fatalf("expected message_received, got %s", notification.Payload.NotificationType)
	}
	if notification.Payload.RecipientUserID != receiverID {
		t.Fatalf("expected recipient %s, got %s", receiverID, notification.Payload.RecipientUserID)
	}
	if notification.Payload.ActorUserID != senderID {
		t.Fatalf("expected actor %s, got %s", senderID, notification.Payload.ActorUserID)
	}
	if notification.Payload.ConversationID != senderID {
		t.Fatalf("expected conversation %s, got %s", senderID, notification.Payload.ConversationID)
	}
	if notification.Payload.Preview != "notification hello" {
		t.Fatalf("expected preview notification hello, got %s", notification.Payload.Preview)
	}
}

func TestMessageNotificationSuppressedWhenReceiverActiveInChat(t *testing.T) {
	t.Parallel()

	timestamp := time.Now().UnixNano()
	tokenSender := registerOrLogin(t, fmt.Sprintf("notify_active_sender_%d", timestamp), "123456")
	tokenReceiver := registerOrLogin(t, fmt.Sprintf("notify_active_receiver_%d", timestamp), "123456")

	senderID := extractUserIDFromJWT(t, tokenSender)
	receiverID := extractUserIDFromJWT(t, tokenReceiver)

	connSender := connectWS(t, tokenSender)
	defer connSender.Close()
	connReceiver := connectWS(t, tokenReceiver)
	defer connReceiver.Close()

	if err := connReceiver.WriteJSON(gatewayMessage{
		Type: "chat_opened",
		Payload: map[string]string{
			"current_chat_id": senderID,
		},
	}); err != nil {
		t.Fatalf("failed to open sender chat: %v", err)
	}
	waitForPresenceCurrentChat(t, receiverID, senderID)

	if err := connSender.WriteJSON(gatewayMessage{
		Type: "chat_message",
		Payload: chatPayload{
			ReceiverID: receiverID,
			Content:    "no notification please",
		},
	}); err != nil {
		t.Fatalf("failed to send chat message: %v", err)
	}

	_ = readMessageWithRetry(t, connSender)
	_ = readMessageWithRetry(t, connReceiver)

	_ = connReceiver.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	var got wsNotificationMessage
	if err := connReceiver.ReadJSON(&got); err == nil && got.Type == "notification" {
		t.Fatalf("expected no notification, got %+v", got)
	}
}

func TestMessageNotificationPublishedWhenReceiverOffline(t *testing.T) {
	t.Parallel()

	timestamp := time.Now().UnixNano()
	tokenSender := registerOrLogin(t, fmt.Sprintf("notify_offline_sender_%d", timestamp), "123456")
	tokenReceiver := registerOrLogin(t, fmt.Sprintf("notify_offline_receiver_%d", timestamp), "123456")

	senderID := extractUserIDFromJWT(t, tokenSender)
	receiverID := extractUserIDFromJWT(t, tokenReceiver)

	connSender := connectWS(t, tokenSender)
	defer connSender.Close()

	channel := subscribeNotifications(t)
	defer channel.Close()

	if err := connSender.WriteJSON(gatewayMessage{
		Type: "chat_message",
		Payload: chatPayload{
			ReceiverID: receiverID,
			Content:    "offline notification",
		},
	}); err != nil {
		t.Fatalf("failed to send chat message: %v", err)
	}

	_ = readMessageWithRetry(t, connSender)

	notification := waitForNotificationPublish(t, channel, func(notification wsNotificationMessage) bool {
		return notification.Payload.NotificationType == "message_received" &&
			notification.Payload.ActorUserID == senderID &&
			notification.Payload.RecipientUserID == receiverID &&
			notification.Payload.Preview == "offline notification"
	})
	if notification.Payload.NotificationType != "message_received" {
		t.Fatalf("expected message_received, got %s", notification.Payload.NotificationType)
	}
	if notification.Payload.ActorUserID != senderID {
		t.Fatalf("expected actor %s, got %s", senderID, notification.Payload.ActorUserID)
	}
	if notification.Payload.RecipientUserID != receiverID {
		t.Fatalf("expected recipient %s, got %s", receiverID, notification.Payload.RecipientUserID)
	}
}

func registerOrLogin(t *testing.T, username, password string) string {
	return registerLoginWithRole(t, username, password, "")
}

func registerAdminOrLogin(t *testing.T, username, password string) string {
	return registerLoginWithRole(t, username, password, "ADMIN")
}

func frontendToken(t *testing.T) string {
	t.Helper()

	frontendReaderTokenOnce.Do(func() {
		frontendReaderToken = registerLoginWithRole(t, "frontend_reader_user", "123456", "")
		if frontendReaderToken == "" {
			frontendReaderTokenErr = errors.New("empty frontend token")
		}
	})
	if frontendReaderTokenErr != nil {
		t.Fatalf("failed to initialize frontend token: %v", frontendReaderTokenErr)
	}
	return frontendReaderToken
}

func adminToken(t *testing.T) string {
	t.Helper()

	adminReaderTokenOnce.Do(func() {
		adminReaderToken = registerLoginWithRole(t, "frontend_admin_user", "123456", "ADMIN")
		if adminReaderToken == "" {
			adminReaderTokenErr = errors.New("empty admin token")
		}
	})
	if adminReaderTokenErr != nil {
		t.Fatalf("failed to initialize admin token: %v", adminReaderTokenErr)
	}
	return adminReaderToken
}

func registerLoginWithRole(t *testing.T, username, password, role string) string {
	t.Helper()

	requestBody := map[string]string{"username": username, "password": password}
	if role != "" {
		requestBody["role"] = role
	}
	body, _ := json.Marshal(requestBody)

	registerResp, err := http.Post(gatewayBaseURL+"/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for integration test: %v", err)
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

	loginResp, err := http.Post(gatewayBaseURL+"/auth/login", "application/json", bytes.NewBuffer(body))
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

func extractRoleFromJWT(t *testing.T, token string) string {
	t.Helper()

	claims := extractJWTClaims(t, token)
	role, _ := claims["role"].(string)
	if role == "" {
		t.Fatalf("JWT missing role claim")
	}
	return role
}

func connectWS(t *testing.T, token string) *websocket.Conn {
	t.Helper()

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	conn, _, err := websocket.DefaultDialer.Dial(websocketURL(gatewayBaseURL, "/ws"), headers)
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

func readRealtimeEventWithRetry(t *testing.T, conn *websocket.Conn) wsRealtimeEvent {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))

	for {
		var event wsRealtimeEvent
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatalf("failed reading websocket event: %v", err)
		}
		if event.Type != "" {
			return event
		}
	}
}

func readNotificationWithRetry(t *testing.T, conn *websocket.Conn) wsNotificationMessage {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))

	for {
		var raw map[string]json.RawMessage
		if err := conn.ReadJSON(&raw); err != nil {
			t.Fatalf("failed reading websocket notification: %v", err)
		}

		var typ string
		if err := json.Unmarshal(raw["type"], &typ); err != nil {
			t.Fatalf("failed decoding websocket type: %v", err)
		}
		if typ != "notification" {
			continue
		}

		var notification wsNotificationMessage
		payload, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("failed to remarshal websocket message: %v", err)
		}
		if err := json.Unmarshal(payload, &notification); err != nil {
			t.Fatalf("failed decoding websocket notification: %v", err)
		}
		return notification
	}
}

func subscribeNotifications(t *testing.T) *redis.PubSub {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6380"
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = client.Close()
	})

	pubsub := client.Subscribe(context.Background(), "notifications")
	if _, err := pubsub.Receive(context.Background()); err != nil {
		t.Fatalf("failed to subscribe notifications: %v", err)
	}
	return pubsub
}

func waitForNotificationPublish(t *testing.T, pubsub *redis.PubSub, match func(wsNotificationMessage) bool) wsNotificationMessage {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	channel := pubsub.Channel()
	for time.Now().Before(deadline) {
		select {
		case msg := <-channel:
			var notification wsNotificationMessage
			if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
				t.Fatalf("failed to decode published notification: %v", err)
			}
			if !match(notification) {
				continue
			}
			return notification
		case <-time.After(200 * time.Millisecond):
		}
	}

	t.Fatal("matching notification was not published before timeout")
	return wsNotificationMessage{}
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

func waitForViewedStatus(t *testing.T, token, otherUserID, messageID, expectedStatus string) {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		result, err := getConversation(t, token, otherUserID, "", 0)
		if err != nil {
			t.Fatalf("unexpected HTTP error: %v", err)
		}

		for _, message := range result.Messages {
			if message.ID == messageID && message.ViewedStatus == expectedStatus {
				return
			}
		}

		time.Sleep(150 * time.Millisecond)
	}

	t.Fatalf("message %s did not reach viewed status %s", messageID, expectedStatus)
}

func extractUserIDFromJWT(t *testing.T, token string) string {
	t.Helper()

	claims := extractJWTClaims(t, token)
	id, _ := claims["userId"].(string)
	if id == "" {
		t.Fatalf("JWT missing userId claim")
	}
	return id
}

func extractJWTClaims(t *testing.T, token string) map[string]interface{} {
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

	return claims
}
