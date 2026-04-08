package websocket

import "encoding/json"

type ValidationError struct {
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Message
}

type MessageResponse struct {
	ID           string `json:"id"`
	SenderID     string `json:"sender_id"`
	ReceiverID   string `json:"receiver_id"`
	Content      string `json:"content"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	ViewedStatus string `json:"viewed_status,omitempty"`
}

type GatewayMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ChatMessagePayload struct {
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
}

type ChatInteractionPayload struct {
	TargetUserID  string `json:"target_user_id,omitempty"`
	CurrentChatID string `json:"current_chat_id,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
}

type ErrorResponse struct {
	Type  string          `json:"type"`
	Error ValidationError `json:"error"`
}

const (
	MessageTypeChat             = "chat_message"
	MessageTypeError            = "error"
	MessageTypePresence         = "presence"
	MessageTypeNotification     = "notification"
	MessageTypeTypingStarted    = "typing_started"
	MessageTypeTypingStopped    = "typing_stopped"
	MessageTypeChatOpened       = "chat_opened"
	MessageTypeChatClosed       = "chat_closed"
	MessageTypeMessageDelivered = "message_delivered"
	MessageTypeMessageSeen      = "message_seen"

	ViewedStatusSent      = "sent"
	ViewedStatusDelivered = "delivered"
	ViewedStatusSeen      = "seen"
)

// FriendEvent is the envelope published to Redis by the gateway and consumed by the websocket service.
type FriendEvent struct {
	TargetUserID string          `json:"target_user_id"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
}

// FriendEventMessage is what the websocket service sends to the connected client.
type FriendEventMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type RealtimeEventMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type NotificationPayload struct {
	NotificationType string `json:"notification_type"`
	RecipientUserID  string `json:"recipient_user_id"`
	ActorUserID      string `json:"actor_user_id"`
	EntityID         string `json:"entity_id"`
	ConversationID   string `json:"conversation_id,omitempty"`
	Preview          string `json:"preview,omitempty"`
	OccurredAt       string `json:"occurred_at"`
}

type NotificationMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ChatInteractionEvent struct {
	Type          string `json:"type"`
	ActorUserID   string `json:"actor_user_id"`
	TargetUserID  string `json:"target_user_id"`
	CurrentChatID string `json:"current_chat_id,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
	ViewedStatus  string `json:"viewed_status,omitempty"`
	OccurredAt    string `json:"occurred_at"`
}

type ChatInteractionNotification struct {
	ActorUserID   string `json:"actor_user_id"`
	CurrentChatID string `json:"current_chat_id,omitempty"`
	MessageID     string `json:"message_id,omitempty"`
	ViewedStatus  string `json:"viewed_status,omitempty"`
	OccurredAt    string `json:"occurred_at"`
}

type PresenceLifecycleEvent struct {
	UserID        string `json:"user_id"`
	Type          string `json:"type"`
	CurrentChatID string `json:"current_chat_id,omitempty"`
	OccurredAt    string `json:"occurred_at"`
}

func (e ChatInteractionEvent) Notification() ChatInteractionNotification {
	return ChatInteractionNotification{
		ActorUserID:   e.ActorUserID,
		CurrentChatID: e.CurrentChatID,
		MessageID:     e.MessageID,
		ViewedStatus:  e.ViewedStatus,
		OccurredAt:    e.OccurredAt,
	}
}
