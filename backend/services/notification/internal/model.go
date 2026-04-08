package notification

import "encoding/json"

const (
	MessageTypeNotification = "notification"

	NotificationTypeFriendRequestReceived = "friend_request_received"
	NotificationTypeMessageReceived       = "message_received"

	PresenceStatusOnline = "online"
)

type FriendRequestIntent struct {
	Type            string `json:"type"`
	EventID         string `json:"event_id"`
	SenderID        string `json:"sender_id"`
	ReceiverID      string `json:"receiver_id"`
	FriendRequestID string `json:"friend_request_id"`
	OccurredAt      string `json:"occurred_at"`
}

type MessageIntent struct {
	Type       string `json:"type"`
	EventID    string `json:"event_id"`
	MessageID  string `json:"message_id"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp,omitempty"`
	OccurredAt string `json:"occurred_at"`
}

type PresenceSnapshot struct {
	UserID        string `json:"user_id"`
	Status        string `json:"status"`
	CurrentChatID string `json:"current_chat_id,omitempty"`
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
