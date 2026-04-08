package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type Service struct {
	repo              Repository
	notificationChann string
}

func NewService(repo Repository, notificationChannel string) *Service {
	return &Service{repo: repo, notificationChann: notificationChannel}
}

func (s *Service) HandleFriendRequestIntent(ctx context.Context, intent FriendRequestIntent) error {
	if err := validateFriendRequestIntent(intent); err != nil {
		return err
	}

	return s.publishPayload(ctx, NotificationPayload{
		NotificationType: NotificationTypeFriendRequestReceived,
		RecipientUserID:  intent.ReceiverID,
		ActorUserID:      intent.SenderID,
		EntityID:         intent.FriendRequestID,
		OccurredAt:       intent.OccurredAt,
	})
}

func (s *Service) HandleMessageIntent(ctx context.Context, intent MessageIntent) (bool, error) {
	if err := validateMessageIntent(intent); err != nil {
		return false, err
	}

	presence, err := s.repo.GetPresence(ctx, intent.ReceiverID)
	if err != nil && !errors.Is(err, ErrPresenceNotFound) {
		return false, err
	}
	if shouldSuppressMessageNotification(presence, err == nil, intent.SenderID) {
		return false, nil
	}

	return true, s.publishPayload(ctx, NotificationPayload{
		NotificationType: NotificationTypeMessageReceived,
		RecipientUserID:  intent.ReceiverID,
		ActorUserID:      intent.SenderID,
		EntityID:         intent.MessageID,
		ConversationID:   intent.SenderID,
		Preview:          intent.Content,
		OccurredAt:       intent.OccurredAt,
	})
}

func shouldSuppressMessageNotification(presence PresenceSnapshot, found bool, senderID string) bool {
	if !found {
		return false
	}

	return presence.Status == PresenceStatusOnline && presence.CurrentChatID == senderID
}

func (s *Service) publishPayload(ctx context.Context, payload NotificationPayload) error {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return s.repo.PublishNotification(ctx, s.notificationChann, NotificationMessage{
		Type:    MessageTypeNotification,
		Payload: rawPayload,
	})
}

func validateFriendRequestIntent(intent FriendRequestIntent) error {
	if strings.TrimSpace(intent.Type) != NotificationTypeFriendRequestReceived {
		return errors.New("invalid friend request notification type")
	}
	if strings.TrimSpace(intent.SenderID) == "" {
		return errors.New("sender_id is required")
	}
	if strings.TrimSpace(intent.ReceiverID) == "" {
		return errors.New("receiver_id is required")
	}
	if strings.TrimSpace(intent.FriendRequestID) == "" {
		return errors.New("friend_request_id is required")
	}
	if strings.TrimSpace(intent.OccurredAt) == "" {
		return errors.New("occurred_at is required")
	}
	return nil
}

func validateMessageIntent(intent MessageIntent) error {
	if strings.TrimSpace(intent.Type) != NotificationTypeMessageReceived {
		return errors.New("invalid message notification type")
	}
	if strings.TrimSpace(intent.MessageID) == "" {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(intent.SenderID) == "" {
		return errors.New("sender_id is required")
	}
	if strings.TrimSpace(intent.ReceiverID) == "" {
		return errors.New("receiver_id is required")
	}
	if strings.TrimSpace(intent.Content) == "" {
		return errors.New("content is required")
	}
	if strings.TrimSpace(intent.OccurredAt) == "" {
		return errors.New("occurred_at is required")
	}
	return nil
}
