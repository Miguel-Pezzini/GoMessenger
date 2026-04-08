package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	CategoryAudit = "audit"
	CategoryError = "error"

	StatusSuccess = "success"
	StatusFailure = "failure"
)

type Event struct {
	EventID      string         `json:"event_id" bson:"event_id"`
	EventType    string         `json:"event_type" bson:"event_type"`
	Category     string         `json:"category" bson:"category"`
	Service      string         `json:"service" bson:"service"`
	ActorUserID  string         `json:"actor_user_id,omitempty" bson:"actor_user_id,omitempty"`
	TargetUserID string         `json:"target_user_id,omitempty" bson:"target_user_id,omitempty"`
	EntityType   string         `json:"entity_type,omitempty" bson:"entity_type,omitempty"`
	EntityID     string         `json:"entity_id,omitempty" bson:"entity_id,omitempty"`
	OccurredAt   string         `json:"occurred_at" bson:"occurred_at"`
	Status       string         `json:"status" bson:"status"`
	Message      string         `json:"message" bson:"message"`
	Metadata     map[string]any `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

func (e Event) Normalize() Event {
	if e.EventID == "" {
		e.EventID = primitive.NewObjectID().Hex()
	}
	if e.OccurredAt == "" {
		e.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	return e
}

func (e Event) Validate() error {
	if e.EventType == "" {
		return errors.New("event_type is required")
	}
	if e.Category == "" {
		return errors.New("category is required")
	}
	if e.Service == "" {
		return errors.New("service is required")
	}
	if e.Status == "" {
		return errors.New("status is required")
	}
	if e.Message == "" {
		return errors.New("message is required")
	}
	if e.OccurredAt == "" {
		return errors.New("occurred_at is required")
	}
	if _, err := time.Parse(time.RFC3339, e.OccurredAt); err != nil {
		return fmt.Errorf("occurred_at must be RFC3339: %w", err)
	}
	return nil
}
