package presence

import (
	"context"
	"errors"
)

var ErrPresenceNotFound = errors.New("presence not found")

type Service struct {
	repo          Repository
	updateChannel string
}

func NewService(repo Repository, updateChannel string) *Service {
	return &Service{repo: repo, updateChannel: updateChannel}
}

func (s *Service) HandleLifecycleEvent(ctx context.Context, event LifecycleEvent) (Presence, error) {
	if event.UserID == "" {
		return Presence{}, errors.New("user_id is required")
	}

	presence := Presence{
		UserID:        event.UserID,
		CurrentChatID: event.CurrentChatID,
	}

	switch event.Type {
	case LifecycleEventConnected:
		presence.Status = StatusOnline
	case LifecycleEventChatOpened:
		if event.CurrentChatID == "" {
			return Presence{}, errors.New("current_chat_id is required for chat_opened events")
		}
		presence.Status = StatusOnline
	case LifecycleEventChatClosed:
		presence.Status = StatusOnline
		presence.CurrentChatID = ""
	case LifecycleEventDisconnected:
		presence.Status = StatusOffline
		presence.CurrentChatID = ""
		occurredAt := event.OccurredAt
		if occurredAt.IsZero() {
			return Presence{}, errors.New("occurred_at is required for disconnect events")
		}
		presence.LastSeen = &occurredAt
	default:
		return Presence{}, errors.New("unsupported lifecycle event type")
	}

	if err := s.repo.Save(ctx, presence); err != nil {
		return Presence{}, err
	}
	if err := s.repo.Publish(ctx, s.updateChannel, presence); err != nil {
		return Presence{}, err
	}

	return presence, nil
}

func (s *Service) GetPresence(ctx context.Context, userID string) (Presence, error) {
	if userID == "" {
		return Presence{}, errors.New("user_id is required")
	}

	return s.repo.Get(ctx, userID)
}
