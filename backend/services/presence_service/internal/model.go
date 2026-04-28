package presence

import "time"

const (
	StatusOnline  = "online"
	StatusOffline = "offline"

	LifecycleEventConnected    = "connected"
	LifecycleEventDisconnected = "disconnected"
	LifecycleEventChatOpened   = "chat_opened"
	LifecycleEventChatClosed   = "chat_closed"
)

type Presence struct {
	UserID        string     `json:"user_id"`
	Status        string     `json:"status"`
	LastSeen      *time.Time `json:"last_seen,omitempty"`
	CurrentChatID string     `json:"current_chat_id,omitempty"`
}

type ActiveUser struct {
	UserID        string     `json:"user_id"`
	Username      string     `json:"username,omitempty"`
	Status        string     `json:"status"`
	LastSeen      *time.Time `json:"last_seen,omitempty"`
	CurrentChatID string     `json:"current_chat_id,omitempty"`
}

type ActiveUsersResponse struct {
	Users []ActiveUser `json:"users"`
	Count int          `json:"count"`
}

type LifecycleEvent struct {
	UserID        string    `json:"user_id"`
	Type          string    `json:"type"`
	CurrentChatID string    `json:"current_chat_id,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}
