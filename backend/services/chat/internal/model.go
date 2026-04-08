package chat

const (
	ViewedStatusSent      = "sent"
	ViewedStatusDelivered = "delivered"
	ViewedStatusSeen      = "seen"
)

type MessageRequest struct {
	StreamID   string `json:"-"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

type MessageDB struct {
	Id           string `json:"id" bson:"-"`
	StreamID     string `json:"-" bson:"stream_id"`
	SenderID     string `json:"sender_id" bson:"sender_id"`
	ReceiverID   string `json:"receiver_id" bson:"receiver_id"`
	Content      string `json:"content" bson:"content"`
	Timestamp    int64  `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
	ViewedStatus string `json:"viewed_status,omitempty" bson:"viewed_status,omitempty"`
}

type MessageResponse struct {
	Id           string `json:"id"`
	SenderID     string `json:"sender_id"`
	ReceiverID   string `json:"receiver_id"`
	Content      string `json:"content"`
	Timestamp    int64  `json:"timestamp,omitempty"`
	ViewedStatus string `json:"viewed_status,omitempty"`
}

type ConversationResponse struct {
	Messages []MessageResponse `json:"messages"`
	HasMore  bool              `json:"has_more"`
}

type InteractionEvent struct {
	Type         string `json:"type"`
	ActorUserID  string `json:"actor_user_id"`
	TargetUserID string `json:"target_user_id"`
	MessageID    string `json:"message_id,omitempty"`
	ViewedStatus string `json:"viewed_status,omitempty"`
}

func NormalizeViewedStatus(status string) string {
	switch status {
	case ViewedStatusDelivered:
		return ViewedStatusDelivered
	case ViewedStatusSeen:
		return ViewedStatusSeen
	default:
		return ViewedStatusSent
	}
}

func ViewedStatusRank(status string) int {
	switch NormalizeViewedStatus(status) {
	case ViewedStatusDelivered:
		return 1
	case ViewedStatusSeen:
		return 2
	default:
		return 0
	}
}
