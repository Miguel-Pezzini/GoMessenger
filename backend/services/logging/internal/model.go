package logging

import "github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"

type StoredEvent struct {
	StreamID    string `json:"stream_id" bson:"stream_id"`
	audit.Event `bson:",inline"`
}

type LogFilter struct {
	Limit       int
	Service     string
	Category    string
	Status      string
	EventType   string
	ActorUserID string
	Query       string
}
