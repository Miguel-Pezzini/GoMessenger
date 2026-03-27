package domain

import "encoding/json"

type ValidationError struct {
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Message
}

type MessageResponse struct {
	ID         string `json:"id"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
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

type ErrorResponse struct {
	Type  string          `json:"type"`
	Error ValidationError `json:"error"`
}

const (
	MessageTypeChat  = "chat_message"
	MessageTypeError = "error"
)
