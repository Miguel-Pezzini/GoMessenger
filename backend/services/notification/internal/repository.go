package notification

import "context"

type Repository interface {
	GetPresence(ctx context.Context, userID string) (PresenceSnapshot, error)
	PublishNotification(ctx context.Context, channel string, notification NotificationMessage) error
}
