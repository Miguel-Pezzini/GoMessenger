package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/security"
	"github.com/gorilla/websocket"
)

type handlerRepositoryStub struct {
	mu             sync.Mutex
	addErr         error
	publishErr     error
	streamPayloads []string
	publishCalls   []publishCall
	subscribers    map[string]func(string)
}

type auditPublisherStub struct {
	mu     sync.Mutex
	events []audit.Event
}

type publishCall struct {
	channel string
	payload string
}

type socketWriterStub struct {
	writeJSON    func(v any) error
	writeControl func(messageType int, data []byte, deadline time.Time) error
	close        func() error
}

func (r *handlerRepositoryStub) AddToStream(streamName, payload string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streamPayloads = append(r.streamPayloads, streamName+"|"+payload)
	return r.addErr
}

func (r *handlerRepositoryStub) Publish(channelName, payload string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.publishCalls = append(r.publishCalls, publishCall{channel: channelName, payload: payload})
	return r.publishErr
}

func (r *handlerRepositoryStub) Subscribe(channel string, handler func(payload string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.subscribers == nil {
		r.subscribers = make(map[string]func(string))
	}
	r.subscribers[channel] = handler
}

func (r *handlerRepositoryStub) publishToSubscriber(channel, payload string) {
	r.mu.Lock()
	handler := r.subscribers[channel]
	r.mu.Unlock()
	if handler != nil {
		handler(payload)
	}
}

func (p *auditPublisherStub) Publish(_ context.Context, event audit.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event.Normalize())
	return nil
}

func (s *socketWriterStub) WriteJSON(v any) error {
	if s.writeJSON != nil {
		return s.writeJSON(v)
	}
	return nil
}

func (s *socketWriterStub) WriteControl(messageType int, data []byte, deadline time.Time) error {
	if s.writeControl != nil {
		return s.writeControl(messageType, data, deadline)
	}
	return nil
}

func (s *socketWriterStub) Close() error {
	if s.close != nil {
		return s.close()
	}
	return nil
}

func TestHandleConnectionRejectsMissingUserID(t *testing.T) {
	handler := NewHandler(NewService(&handlerRepositoryStub{}, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	handler.HandleConnection(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing user id") {
		t.Fatalf("expected missing user id response, got %q", rec.Body.String())
	}
}

func TestHandleConnectionRejectsDisallowedOrigin(t *testing.T) {
	handler := NewHandler(NewService(&handlerRepositoryStub{}, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator([]string{"http://allowed.test"}))
	server := newWebsocketTestServer(handler)
	defer server.Close()

	wsURL := websocketURL(t, server.URL)
	headers := http.Header{}
	headers.Set("X-User-ID", "user-a")
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

func TestHandleConnectionReturnsErrorForMalformedMessage(t *testing.T) {
	handler := NewHandler(NewService(&handlerRepositoryStub{}, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	server := newWebsocketTestServer(handler)
	defer server.Close()

	conn := dialWebsocket(t, server, "user-auth")
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"chat_message","payload":`)); err != nil {
		t.Fatalf("failed to write raw websocket message: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got ErrorResponse
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("failed to read websocket response: %v", err)
	}
	if got.Error.Message != "invalid message payload" {
		t.Fatalf("unexpected validation error: %s", got.Error.Message)
	}
}

func TestHandleConnectionReturnsValidationErrorsForInvalidChatPayloadShape(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	server := newWebsocketTestServer(handler)
	defer server.Close()

	conn := dialWebsocket(t, server, "user-auth")
	defer conn.Close()

	err := conn.WriteJSON(map[string]any{
		"type":    MessageTypeChat,
		"payload": map[string]any{"receiver_id": "user-b", "content": "hello", "extra": true},
	})
	if err != nil {
		t.Fatalf("failed to write websocket message: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got ErrorResponse
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("failed to read websocket response: %v", err)
	}
	if got.Error.Message != "invalid chat payload" {
		t.Fatalf("unexpected validation error: %s", got.Error.Message)
	}
}

func TestHandleConnectionPublishesPresenceAcrossReconnectAndDisconnect(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	handler.SetPresenceChannel("presence.lifecycle")
	server := newWebsocketTestServer(handler)
	defer server.Close()

	connectAndCloseWebsocket(t, server, "user-a")
	waitForPublishCalls(t, repo, 2)
	assertPresenceSequence(t, repo.publishCalls, []string{"connected", "disconnected"})

	connectAndCloseWebsocket(t, server, "user-a")
	waitForPublishCalls(t, repo, 4)
	assertPresenceSequence(t, repo.publishCalls, []string{"connected", "disconnected", "connected", "disconnected"})

	waitForCondition(t, func() bool {
		handler.clientsM.RLock()
		defer handler.clientsM.RUnlock()
		return len(handler.clients) == 0
	})
}

func TestHandleConnectionReturnsValidationErrorsToClient(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	server := newWebsocketTestServer(handler)
	defer server.Close()

	conn := dialWebsocket(t, server, "user-auth")
	defer conn.Close()

	err := conn.WriteJSON(GatewayMessage{
		Type: MessageTypeChat,
		Payload: mustRawJSON(t, ChatMessagePayload{
			SenderID:   "spoofed-user",
			ReceiverID: "user-b",
			Content:    "hello",
		}),
	})
	if err != nil {
		t.Fatalf("failed to write websocket message: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got ErrorResponse
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("failed to read websocket response: %v", err)
	}

	if got.Type != MessageTypeError {
		t.Fatalf("expected error message type, got %s", got.Type)
	}
	if got.Error.Message != "sender_id does not match authenticated user" {
		t.Fatalf("unexpected validation error: %s", got.Error.Message)
	}
}

func TestHandleConnectionClosesWithoutPublishingWhenPresenceFails(t *testing.T) {
	repo := &handlerRepositoryStub{publishErr: errors.New("redis unavailable")}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	handler.SetPresenceChannel("presence.lifecycle")
	server := newWebsocketTestServer(handler)
	defer server.Close()

	connectAndCloseWebsocket(t, server, "user-a")
	waitForPublishCalls(t, repo, 2)

	waitForCondition(t, func() bool {
		handler.clientsM.RLock()
		defer handler.clientsM.RUnlock()
		return len(handler.clients) == 0
	})
}

func TestHandleConnectionPublishesAuditEventsForLifecycle(t *testing.T) {
	repo := &handlerRepositoryStub{}
	auditPublisher := &auditPublisherStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), auditPublisher, security.NewOriginValidator(nil))
	server := newWebsocketTestServer(handler)
	defer server.Close()

	connectAndCloseWebsocket(t, server, "user-a")
	waitForCondition(t, func() bool {
		auditPublisher.mu.Lock()
		defer auditPublisher.mu.Unlock()
		return len(auditPublisher.events) >= 2
	})

	if auditPublisher.events[0].EventType != "websocket.connected" {
		t.Fatalf("expected websocket.connected, got %s", auditPublisher.events[0].EventType)
	}
	if auditPublisher.events[1].EventType != "websocket.disconnected" {
		t.Fatalf("expected websocket.disconnected, got %s", auditPublisher.events[1].EventType)
	}
}

func TestHandleConnectionPublishesTypingEvents(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	handler.SetChatEventsChannel("chat.events")
	server := newWebsocketTestServer(handler)
	defer server.Close()

	conn := dialWebsocket(t, server, "user-a")
	defer conn.Close()

	err := conn.WriteJSON(GatewayMessage{
		Type: MessageTypeTypingStarted,
		Payload: mustRawJSON(t, ChatInteractionPayload{
			TargetUserID:  "user-b",
			CurrentChatID: "user-b",
		}),
	})
	if err != nil {
		t.Fatalf("failed to write websocket message: %v", err)
	}

	waitForPublishCalls(t, repo, 1)

	var event ChatInteractionEvent
	if err := json.Unmarshal([]byte(repo.publishCalls[0].payload), &event); err != nil {
		t.Fatalf("failed to decode chat interaction event: %v", err)
	}
	if event.Type != MessageTypeTypingStarted {
		t.Fatalf("expected typing_started, got %s", event.Type)
	}
	if event.TargetUserID != "user-b" {
		t.Fatalf("expected target user-b, got %s", event.TargetUserID)
	}
}

func TestHandleConnectionPublishesChatOpenAndClosePresenceEvents(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	handler.SetPresenceChannel("presence.lifecycle")
	server := newWebsocketTestServer(handler)
	defer server.Close()

	conn := dialWebsocket(t, server, "user-a")
	defer conn.Close()

	if err := conn.WriteJSON(GatewayMessage{
		Type: MessageTypeChatOpened,
		Payload: mustRawJSON(t, ChatInteractionPayload{
			CurrentChatID: "user-b",
		}),
	}); err != nil {
		t.Fatalf("failed to write chat_opened: %v", err)
	}

	if err := conn.WriteJSON(GatewayMessage{
		Type: MessageTypeChatClosed,
		Payload: mustRawJSON(t, ChatInteractionPayload{
			CurrentChatID: "user-b",
		}),
	}); err != nil {
		t.Fatalf("failed to write chat_closed: %v", err)
	}

	waitForPublishCalls(t, repo, 3)
	assertPresenceSequence(t, repo.publishCalls, []string{"connected", MessageTypeChatOpened, MessageTypeChatClosed})
}

func TestStartChatEventListenerForwardsEventsToTargetUser(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	handler.StartChatEventListener("chat.events")
	server := newWebsocketTestServer(handler)
	defer server.Close()

	connA := dialWebsocket(t, server, "user-a")
	defer connA.Close()
	connB := dialWebsocket(t, server, "user-b")
	defer connB.Close()

	waitForCondition(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return repo.subscribers["chat.events"] != nil
	})

	payload, err := json.Marshal(ChatInteractionEvent{
		Type:          MessageTypeMessageSeen,
		ActorUserID:   "user-b",
		TargetUserID:  "user-b",
		CurrentChatID: "user-a",
		MessageID:     "mongo-id",
		ViewedStatus:  ViewedStatusSeen,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	repo.publishToSubscriber("chat.events", string(payload))

	_ = connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got RealtimeEventMessage
	if err := connB.ReadJSON(&got); err != nil {
		t.Fatalf("failed to read forwarded event: %v", err)
	}

	if got.Type != MessageTypeMessageSeen {
		t.Fatalf("expected type %s, got %s", MessageTypeMessageSeen, got.Type)
	}

	var notification ChatInteractionNotification
	if err := json.Unmarshal(got.Payload, &notification); err != nil {
		t.Fatalf("failed to unmarshal notification: %v", err)
	}
	if notification.MessageID != "mongo-id" {
		t.Fatalf("expected message id mongo-id, got %s", notification.MessageID)
	}
	if notification.ViewedStatus != ViewedStatusSeen {
		t.Fatalf("expected viewed status %s, got %s", ViewedStatusSeen, notification.ViewedStatus)
	}
}

func TestStartPubSubListenerReleasesClientsLockBeforeWrite(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	writeStarted := make(chan struct{})
	releaseWrite := make(chan struct{})

	handler.clients["user-b"] = &clientConn{
		conn: &socketWriterStub{
			writeJSON: func(v any) error {
				close(writeStarted)
				<-releaseWrite
				return nil
			},
		},
	}

	handler.StartPubSubListener("chat.events")
	waitForSubscriber(t, repo, "chat.events")

	done := make(chan struct{})
	go func() {
		repo.publishToSubscriber("chat.events", string(mustJSON(t, MessageResponse{
			SenderID:   "user-a",
			ReceiverID: "user-b",
			Content:    "hello",
		})))
		close(done)
	}()

	waitForChannel(t, writeStarted)

	lockAcquired := make(chan struct{})
	go func() {
		handler.clientsM.Lock()
		close(lockAcquired)
		handler.clientsM.Unlock()
	}()

	waitForChannel(t, lockAcquired)
	close(releaseWrite)
	waitForChannel(t, done)
}

func TestStartPubSubListenerServesSenderWhileReceiverWriteBlocked(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	receiverWriteStarted := make(chan struct{})
	releaseReceiverWrite := make(chan struct{})
	senderWrites := make(chan any, 1)

	handler.clients["user-a"] = &clientConn{
		conn: &socketWriterStub{
			writeJSON: func(v any) error {
				senderWrites <- v
				return nil
			},
		},
	}
	handler.clients["user-b"] = &clientConn{
		conn: &socketWriterStub{
			writeJSON: func(v any) error {
				close(receiverWriteStarted)
				<-releaseReceiverWrite
				return nil
			},
		},
	}

	handler.StartPubSubListener("chat.events")
	waitForSubscriber(t, repo, "chat.events")

	done := make(chan struct{})
	go func() {
		repo.publishToSubscriber("chat.events", string(mustJSON(t, MessageResponse{
			SenderID:   "user-a",
			ReceiverID: "user-b",
			Content:    "hello",
		})))
		close(done)
	}()

	waitForChannel(t, receiverWriteStarted)

	got, ok := waitForWrite(t, senderWrites).(MessageResponse)
	if !ok {
		t.Fatalf("expected sender write to carry MessageResponse")
	}
	if got.SenderID != "user-a" || got.ReceiverID != "user-b" || got.Content != "hello" {
		t.Fatalf("unexpected fan-out payload: %+v", got)
	}

	select {
	case <-done:
		t.Fatal("publish returned before blocked receiver write was released")
	default:
	}

	close(releaseReceiverWrite)
	waitForChannel(t, done)
}

func TestStartFriendEventListenerServesOtherUsersWhileOneWriteIsBlocked(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	slowWriteStarted := make(chan struct{})
	releaseSlowWrite := make(chan struct{})
	fastWrites := make(chan any, 1)

	handler.clients["slow-user"] = &clientConn{
		conn: &socketWriterStub{
			writeJSON: func(v any) error {
				close(slowWriteStarted)
				<-releaseSlowWrite
				return nil
			},
		},
	}
	handler.clients["fast-user"] = &clientConn{
		conn: &socketWriterStub{
			writeJSON: func(v any) error {
				fastWrites <- v
				return nil
			},
		},
	}

	handler.StartFriendEventListener("friend.events")
	waitForSubscriber(t, repo, "friend.events")

	slowDone := make(chan struct{})
	go func() {
		repo.publishToSubscriber("friend.events", string(mustJSON(t, FriendEvent{
			TargetUserID: "slow-user",
			Type:         "friend_request_received",
			Payload:      json.RawMessage(`{"id":"req-1"}`),
		})))
		close(slowDone)
	}()

	waitForChannel(t, slowWriteStarted)

	fastDone := make(chan struct{})
	go func() {
		repo.publishToSubscriber("friend.events", string(mustJSON(t, FriendEvent{
			TargetUserID: "fast-user",
			Type:         "friend_request_received",
			Payload:      json.RawMessage(`{"id":"req-2"}`),
		})))
		close(fastDone)
	}()

	got, ok := waitForWrite(t, fastWrites).(FriendEventMessage)
	if !ok {
		t.Fatalf("expected friend event message for fast user")
	}
	if got.Type != "friend_request_received" {
		t.Fatalf("expected friend_request_received, got %s", got.Type)
	}

	waitForChannel(t, fastDone)
	close(releaseSlowWrite)
	waitForChannel(t, slowDone)
}

func TestStartChatEventListenerServesOtherUsersWhileOneWriteIsBlocked(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	slowWriteStarted := make(chan struct{})
	releaseSlowWrite := make(chan struct{})
	fastWrites := make(chan any, 1)

	handler.clients["slow-user"] = &clientConn{
		conn: &socketWriterStub{
			writeJSON: func(v any) error {
				close(slowWriteStarted)
				<-releaseSlowWrite
				return nil
			},
		},
	}
	handler.clients["fast-user"] = &clientConn{
		conn: &socketWriterStub{
			writeJSON: func(v any) error {
				fastWrites <- v
				return nil
			},
		},
	}

	handler.StartChatEventListener("chat.events")
	waitForSubscriber(t, repo, "chat.events")

	slowDone := make(chan struct{})
	go func() {
		repo.publishToSubscriber("chat.events", string(mustJSON(t, ChatInteractionEvent{
			Type:         MessageTypeMessageDelivered,
			ActorUserID:  "actor-a",
			TargetUserID: "slow-user",
			MessageID:    "msg-1",
			ViewedStatus: ViewedStatusDelivered,
			OccurredAt:   time.Now().UTC().Format(time.RFC3339),
		})))
		close(slowDone)
	}()

	waitForChannel(t, slowWriteStarted)

	fastDone := make(chan struct{})
	go func() {
		repo.publishToSubscriber("chat.events", string(mustJSON(t, ChatInteractionEvent{
			Type:         MessageTypeMessageSeen,
			ActorUserID:  "actor-b",
			TargetUserID: "fast-user",
			MessageID:    "msg-2",
			ViewedStatus: ViewedStatusSeen,
			OccurredAt:   time.Now().UTC().Format(time.RFC3339),
		})))
		close(fastDone)
	}()

	got, ok := waitForWrite(t, fastWrites).(RealtimeEventMessage)
	if !ok {
		t.Fatalf("expected realtime event message for fast user")
	}
	if got.Type != MessageTypeMessageSeen {
		t.Fatalf("expected %s, got %s", MessageTypeMessageSeen, got.Type)
	}

	waitForChannel(t, fastDone)
	close(releaseSlowWrite)
	waitForChannel(t, slowDone)
}

func TestStartNotificationListenerForwardsOnlyToRecipient(t *testing.T) {
	repo := &handlerRepositoryStub{}
	handler := NewHandler(NewService(repo, "chat-stream"), &auditPublisherStub{}, security.NewOriginValidator(nil))
	handler.StartNotificationListener("notifications")
	server := newWebsocketTestServer(handler)
	defer server.Close()

	connRecipient := dialWebsocket(t, server, "user-b")
	defer connRecipient.Close()
	connOther := dialWebsocket(t, server, "user-c")
	defer connOther.Close()

	waitForSubscriber(t, repo, "notifications")

	repo.publishToSubscriber("notifications", string(mustJSON(t, NotificationMessage{
		Type: MessageTypeNotification,
		Payload: mustRawJSON(t, NotificationPayload{
			NotificationType: "message_received",
			RecipientUserID:  "user-b",
			ActorUserID:      "user-a",
			EntityID:         "msg-1",
			ConversationID:   "user-a",
			Preview:          "hello",
			OccurredAt:       time.Now().UTC().Format(time.RFC3339),
		}),
	})))

	_ = connRecipient.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got NotificationMessage
	if err := connRecipient.ReadJSON(&got); err != nil {
		t.Fatalf("failed to read notification: %v", err)
	}
	if got.Type != MessageTypeNotification {
		t.Fatalf("expected type %s, got %s", MessageTypeNotification, got.Type)
	}

	_ = connOther.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var unexpected NotificationMessage
	if err := connOther.ReadJSON(&unexpected); err == nil {
		t.Fatalf("expected no notification for other user, got %+v", unexpected)
	}
}

func newWebsocketTestServer(handler *Handler) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.HandleConnection)
	return httptest.NewServer(mux)
}

func connectAndCloseWebsocket(t *testing.T, server *httptest.Server, userID string) {
	t.Helper()

	conn := dialWebsocket(t, server, userID)
	if err := conn.Close(); err != nil {
		t.Fatalf("failed to close websocket: %v", err)
	}
}

func dialWebsocket(t *testing.T, server *httptest.Server, userID string) *websocket.Conn {
	t.Helper()

	wsURL := websocketURL(t, server.URL)
	headers := http.Header{}
	if userID != "" {
		headers.Set("X-User-ID", userID)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	return conn
}

func websocketURL(t *testing.T, serverURL string) string {
	t.Helper()

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse server url: %v", err)
	}
	parsed.Scheme = "ws"
	parsed.Path = "/ws"
	return parsed.String()
}

func waitForSubscriber(t *testing.T, repo *handlerRepositoryStub, channel string) {
	t.Helper()

	waitForCondition(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return repo.subscribers[channel] != nil
	})
}

func waitForPublishCalls(t *testing.T, repo *handlerRepositoryStub, expected int) {
	t.Helper()
	waitForCondition(t, func() bool {
		repo.mu.Lock()
		defer repo.mu.Unlock()
		return len(repo.publishCalls) >= expected
	})
}

func waitForCondition(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("condition not reached before timeout")
}

func waitForChannel(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("channel was not signaled before timeout")
	}
}

func waitForWrite(t *testing.T, ch <-chan any) any {
	t.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("write was not observed before timeout")
		return nil
	}
}

func assertPresenceSequence(t *testing.T, calls []publishCall, expectedTypes []string) {
	t.Helper()

	if len(calls) < len(expectedTypes) {
		t.Fatalf("expected at least %d publish calls, got %d", len(expectedTypes), len(calls))
	}

	for i, expectedType := range expectedTypes {
		var event PresenceLifecycleEvent
		if err := json.Unmarshal([]byte(calls[i].payload), &event); err != nil {
			t.Fatalf("failed to decode presence payload %d: %v", i, err)
		}
		if event.Type != expectedType {
			t.Fatalf("expected event %d type %s, got %s", i, expectedType, event.Type)
		}
	}
}

func mustRawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return data
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal json payload: %v", err)
	}
	return data
}
