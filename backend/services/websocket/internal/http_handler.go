package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/security"
	"github.com/gorilla/websocket"
)

type Handler struct {
	service           *Service
	auditPublisher    audit.Publisher
	presenceChannel   string
	chatEventsChannel string
	clients           map[string]*clientConn
	clientsM          sync.RWMutex
	typingTimers      map[string]*time.Timer
	typingTimersM     sync.Mutex
	originValidator   security.OriginValidator
}

type websocketWriter interface {
	WriteJSON(v any) error
	WriteControl(messageType int, data []byte, deadline time.Time) error
	Close() error
}

type clientConn struct {
	conn   websocketWriter
	writeM sync.Mutex
}

func (c *clientConn) WriteJSON(v any) error {
	c.writeM.Lock()
	defer c.writeM.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *clientConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	c.writeM.Lock()
	defer c.writeM.Unlock()
	return c.conn.WriteControl(messageType, data, deadline)
}

func (c *clientConn) Close() error {
	return c.conn.Close()
}

var typingIdleTimeout = 3 * time.Second

func NewHandler(service *Service, auditPublisher audit.Publisher, originValidator security.OriginValidator) *Handler {
	return &Handler{
		service:         service,
		auditPublisher:  auditPublisher,
		clients:         make(map[string]*clientConn),
		typingTimers:    make(map[string]*time.Timer),
		originValidator: originValidator,
	}
}

func (h *Handler) SetPresenceChannel(channel string) {
	h.presenceChannel = channel
}

func (h *Handler) SetChatEventsChannel(channel string) {
	h.chatEventsChannel = channel
}

func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeHTTPJSONError(w, http.StatusUnauthorized, "missing user id")
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.originValidator.IsAllowed,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade websocket connection:", err)
		return
	}
	client := &clientConn{conn: conn}

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	h.clientsM.Lock()
	h.clients[userID] = client
	h.clientsM.Unlock()
	h.publishAudit(r.Context(), audit.Event{
		EventType:   "websocket.connected",
		Category:    audit.CategoryAudit,
		Service:     "websocket",
		ActorUserID: userID,
		EntityType:  "session",
		Status:      audit.StatusSuccess,
		Message:     "websocket connected",
	})

	if h.presenceChannel != "" {
		if err := h.service.PublishPresenceConnected(h.presenceChannel, userID); err != nil {
			log.Println("Failed to publish presence connect event:", err)
		}
	}

	go h.startPingLoop(client)

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Println("Connection closed:", err)
			break
		}

		var gatewayMessage GatewayMessage
		if err := decodeStrictJSON(msgBytes, &gatewayMessage); err != nil {
			log.Println("Failed to decode websocket message:", err)
			if writeErr := h.writeValidationError(client, ValidationError{Message: "invalid message payload"}); writeErr != nil {
				log.Println("Failed to write websocket validation error:", writeErr)
			}
			continue
		}

		if err := h.handleGatewayMessage(r.Context(), userID, client, gatewayMessage); err != nil {
			log.Println("Failed to handle websocket message:", err)
		}
	}

	h.clientsM.Lock()
	delete(h.clients, userID)
	h.clientsM.Unlock()
	h.publishAudit(context.Background(), audit.Event{
		EventType:   "websocket.disconnected",
		Category:    audit.CategoryAudit,
		Service:     "websocket",
		ActorUserID: userID,
		EntityType:  "session",
		Status:      audit.StatusSuccess,
		Message:     "websocket disconnected",
	})

	h.publishTypingStoppedForUser(userID)

	if h.presenceChannel != "" {
		if err := h.service.PublishPresenceDisconnected(h.presenceChannel, userID); err != nil {
			log.Println("Failed to publish presence disconnect event:", err)
		}
	}
}

func (h *Handler) publishAudit(ctx context.Context, event audit.Event) {
	if h.auditPublisher == nil {
		return
	}
	_ = h.auditPublisher.Publish(ctx, event)
}

func (h *Handler) StartPubSubListener(channelName string) {
	go h.service.SubscribeChatChannel(channelName, func(payload string) {
		var msg MessageResponse
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			log.Println("Failed to decode pubsub message:", err)
			return
		}

		h.sendJSONToUsers(msg, msg.ReceiverID, msg.SenderID)
	})
}

func (h *Handler) StartFriendEventListener(channelName string) {
	go h.service.SubscribeChatChannel(channelName, func(payload string) {
		var event FriendEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			log.Println("Error parsing friend event:", err)
			return
		}

		h.sendJSONToUser(event.TargetUserID, FriendEventMessage{
			Type:    event.Type,
			Payload: event.Payload,
		})
	})
}

func (h *Handler) StartChatEventListener(channelName string) {
	go h.service.SubscribeChatChannel(channelName, func(payload string) {
		var event ChatInteractionEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			log.Println("Error parsing chat interaction event:", err)
			return
		}

		payloadBytes, err := json.Marshal(event.Notification())
		if err != nil {
			log.Println("Error marshalling chat interaction event:", err)
			return
		}

		h.sendJSONToUser(event.TargetUserID, RealtimeEventMessage{
			Type:    event.Type,
			Payload: payloadBytes,
		})
	})
}

func (h *Handler) StartNotificationListener(channelName string) {
	go h.service.SubscribeChatChannel(channelName, func(payload string) {
		var message NotificationMessage
		if err := json.Unmarshal([]byte(payload), &message); err != nil {
			log.Println("Error parsing notification message:", err)
			return
		}

		var notification NotificationPayload
		if err := json.Unmarshal(message.Payload, &notification); err != nil {
			log.Println("Error parsing notification payload:", err)
			return
		}

		h.sendJSONToUser(notification.RecipientUserID, message)
	})
}

func (h *Handler) startPingLoop(conn *clientConn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
			log.Println("Ping error, closing connection:", err)
			_ = conn.Close()
			return
		}
	}
}

func (h *Handler) handleGatewayMessage(ctx context.Context, userID string, conn *clientConn, message GatewayMessage) error {
	if strings.TrimSpace(message.Type) == "" {
		return h.writeValidationError(conn, ValidationError{Message: "type is required"})
	}

	switch message.Type {
	case MessageTypeChat:
		return h.handleChatMessage(ctx, userID, conn, message.Payload)
	case MessageTypeTypingStarted:
		return h.handleTypingStarted(userID, conn, message.Payload)
	case MessageTypeTypingStopped:
		return h.handleTypingStopped(userID, conn, message.Payload, false)
	case MessageTypeChatOpened:
		return h.handleChatOpened(userID, conn, message.Payload)
	case MessageTypeChatClosed:
		return h.handleChatClosed(userID, conn, message.Payload)
	case MessageTypeMessageDelivered, MessageTypeMessageSeen:
		return h.handleInteractionEvent(userID, conn, message.Type, message.Payload)
	default:
		return h.writeValidationError(conn, ValidationError{Message: "unsupported message type"})
	}
}

func (h *Handler) handleChatMessage(ctx context.Context, userID string, conn *clientConn, rawPayload json.RawMessage) error {
	var payload ChatMessagePayload
	if err := decodeStrictRawJSON(rawPayload, &payload); err != nil {
		log.Println("Failed to decode chat payload:", err)
		h.publishAudit(ctx, audit.Event{
			EventType:   "chat.payload.invalid",
			Category:    audit.CategoryError,
			Service:     "websocket",
			ActorUserID: userID,
			EntityType:  "message",
			Status:      audit.StatusFailure,
			Message:     "chat payload parsing failed",
			Metadata:    map[string]any{"error": err.Error()},
		})
		return h.writeValidationError(conn, ValidationError{Message: "invalid chat payload"})
	}

	if err := h.service.PersistMessage(userID, payload); err != nil {
		h.publishAudit(ctx, audit.Event{
			EventType:    "chat.enqueue.failed",
			Category:     audit.CategoryError,
			Service:      "websocket",
			ActorUserID:  userID,
			TargetUserID: payload.ReceiverID,
			EntityType:   "message",
			Status:       audit.StatusFailure,
			Message:      "chat message enqueue failed",
			Metadata:     map[string]any{"error": err.Error()},
		})
		return h.writeMaybeValidationError(conn, err)
	}

	h.publishTypingStoppedIfActive(userID, ChatInteractionPayload{
		TargetUserID:  payload.ReceiverID,
		CurrentChatID: payload.ReceiverID,
	})
	return nil
}

func (h *Handler) handleTypingStarted(userID string, conn *clientConn, rawPayload json.RawMessage) error {
	payload, err := decodeInteractionPayload(rawPayload)
	if err != nil {
		return h.writeValidationError(conn, ValidationError{Message: "invalid interaction payload"})
	}

	if err := h.publishChatInteraction(userID, MessageTypeTypingStarted, payload); err != nil {
		return h.writeMaybeValidationError(conn, err)
	}

	h.resetTypingTimer(userID, payload)
	return nil
}

func (h *Handler) handleTypingStopped(userID string, conn *clientConn, rawPayload json.RawMessage, onlyIfActive bool) error {
	payload, err := decodeInteractionPayload(rawPayload)
	if err != nil {
		return h.writeValidationError(conn, ValidationError{Message: "invalid interaction payload"})
	}

	if onlyIfActive && !h.clearTypingTimer(userID, payload.TargetUserID) {
		return nil
	}

	h.clearTypingTimer(userID, payload.TargetUserID)
	return h.writeMaybeValidationError(conn, h.publishChatInteraction(userID, MessageTypeTypingStopped, payload))
}

func (h *Handler) handleChatOpened(userID string, conn *clientConn, rawPayload json.RawMessage) error {
	payload, err := decodeInteractionPayload(rawPayload)
	if err != nil {
		return h.writeValidationError(conn, ValidationError{Message: "invalid interaction payload"})
	}
	if h.presenceChannel == "" {
		return nil
	}
	return h.writeMaybeValidationError(conn, h.service.PublishPresenceChatOpened(h.presenceChannel, userID, payload.CurrentChatID))
}

func (h *Handler) handleChatClosed(userID string, conn *clientConn, rawPayload json.RawMessage) error {
	payload, err := decodeInteractionPayload(rawPayload)
	if err != nil {
		return h.writeValidationError(conn, ValidationError{Message: "invalid interaction payload"})
	}

	h.publishTypingStoppedIfActive(userID, payload)
	if h.presenceChannel == "" {
		return nil
	}
	return h.writeMaybeValidationError(conn, h.service.PublishPresenceChatClosed(h.presenceChannel, userID))
}

func (h *Handler) handleInteractionEvent(userID string, conn *clientConn, eventType string, rawPayload json.RawMessage) error {
	payload, err := decodeInteractionPayload(rawPayload)
	if err != nil {
		return h.writeValidationError(conn, ValidationError{Message: "invalid interaction payload"})
	}

	return h.writeMaybeValidationError(conn, h.publishChatInteraction(userID, eventType, payload))
}

func (h *Handler) publishChatInteraction(userID, eventType string, payload ChatInteractionPayload) error {
	if h.chatEventsChannel == "" {
		return ValidationError{Message: "chat events channel is not configured"}
	}

	return h.service.PublishChatInteraction(h.chatEventsChannel, userID, eventType, payload)
}

func (h *Handler) writeMaybeValidationError(conn *clientConn, err error) error {
	if err == nil {
		return nil
	}
	if validationErr, ok := err.(ValidationError); ok {
		return h.writeValidationError(conn, validationErr)
	}
	return err
}

func (h *Handler) writeValidationError(conn *clientConn, validationErr ValidationError) error {
	if conn == nil {
		return nil
	}
	if err := conn.WriteJSON(ErrorResponse{
		Type:  MessageTypeError,
		Error: validationErr,
	}); err != nil {
		return err
	}
	return nil
}

func (h *Handler) sendJSONToUser(userID string, message any) {
	h.sendJSONToUsers(message, userID)
}

func (h *Handler) sendJSONToUsers(message any, userIDs ...string) {
	conns := h.snapshotClients(userIDs...)
	if len(conns) == 0 {
		return
	}
	if len(conns) == 1 {
		_ = conns[0].WriteJSON(message)
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(conns))
	for _, conn := range conns {
		conn := conn
		go func() {
			defer wg.Done()
			_ = conn.WriteJSON(message)
		}()
	}
	wg.Wait()
}

func (h *Handler) snapshotClients(userIDs ...string) []*clientConn {
	h.clientsM.RLock()
	defer h.clientsM.RUnlock()

	conns := make([]*clientConn, 0, len(userIDs))
	for _, userID := range userIDs {
		conn, ok := h.clients[userID]
		if !ok {
			continue
		}
		conns = append(conns, conn)
	}

	return conns
}

func decodeInteractionPayload(rawPayload json.RawMessage) (ChatInteractionPayload, error) {
	var payload ChatInteractionPayload
	if err := decodeStrictRawJSON(rawPayload, &payload); err != nil {
		return ChatInteractionPayload{}, err
	}

	if payload.TargetUserID == "" && payload.CurrentChatID != "" {
		payload.TargetUserID = payload.CurrentChatID
	}

	return payload, nil
}

func (h *Handler) resetTypingTimer(userID string, payload ChatInteractionPayload) {
	targetUserID := payload.TargetUserID
	if targetUserID == "" || h.chatEventsChannel == "" {
		return
	}

	key := typingTimerKey(userID, targetUserID)

	h.typingTimersM.Lock()
	if timer, ok := h.typingTimers[key]; ok {
		timer.Stop()
	}

	timerPayload := payload
	var timer *time.Timer
	timer = time.AfterFunc(typingIdleTimeout, func() {
		h.typingTimersM.Lock()
		current, ok := h.typingTimers[key]
		if ok && current == timer {
			delete(h.typingTimers, key)
		}
		h.typingTimersM.Unlock()
		if !ok {
			return
		}
		if err := h.service.PublishChatInteraction(h.chatEventsChannel, userID, MessageTypeTypingStopped, timerPayload); err != nil {
			log.Println("Failed to publish typing stop event:", err)
		}
	})

	h.typingTimers[key] = timer
	h.typingTimersM.Unlock()
}

func (h *Handler) clearTypingTimer(userID, targetUserID string) bool {
	if targetUserID == "" {
		return false
	}

	key := typingTimerKey(userID, targetUserID)
	h.typingTimersM.Lock()
	defer h.typingTimersM.Unlock()

	timer, ok := h.typingTimers[key]
	if ok {
		delete(h.typingTimers, key)
		timer.Stop()
	}
	return ok
}

func (h *Handler) publishTypingStoppedIfActive(userID string, payload ChatInteractionPayload) {
	targetUserID := payload.TargetUserID
	if targetUserID == "" {
		targetUserID = payload.CurrentChatID
	}
	if targetUserID == "" {
		return
	}

	stopPayload := payload
	stopPayload.TargetUserID = targetUserID
	if !h.clearTypingTimer(userID, targetUserID) {
		return
	}

	if err := h.publishChatInteraction(userID, MessageTypeTypingStopped, stopPayload); err != nil {
		log.Println("Failed to publish typing stop event:", err)
	}
}

func (h *Handler) publishTypingStoppedForUser(userID string) {
	type pendingStop struct {
		targetUserID string
	}

	var pending []pendingStop

	h.typingTimersM.Lock()
	for key, timer := range h.typingTimers {
		parts := strings.SplitN(key, "\x00", 2)
		if len(parts) != 2 || parts[0] != userID {
			continue
		}
		delete(h.typingTimers, key)
		timer.Stop()
		pending = append(pending, pendingStop{targetUserID: parts[1]})
	}
	h.typingTimersM.Unlock()

	for _, stop := range pending {
		if err := h.publishChatInteraction(userID, MessageTypeTypingStopped, ChatInteractionPayload{
			TargetUserID:  stop.targetUserID,
			CurrentChatID: stop.targetUserID,
		}); err != nil {
			log.Println("Failed to publish typing stop event:", err)
		}
	}
}

func typingTimerKey(userID, targetUserID string) string {
	return userID + "\x00" + targetUserID
}

func decodeStrictRawJSON(rawPayload json.RawMessage, dst any) error {
	if len(rawPayload) == 0 {
		return errors.New("missing payload")
	}

	return decodeStrictJSON(rawPayload, dst)
}

func decodeStrictJSON(data []byte, dst any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty body")
		}
		return err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple json values")
		}
		return err
	}

	return nil
}

func writeHTTPJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
