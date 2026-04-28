package presence

import "context"

type Repository interface {
	Save(ctx context.Context, presence Presence) error
	Get(ctx context.Context, userID string) (Presence, error)
	ListActive(ctx context.Context, limit int) ([]Presence, error)
	Publish(ctx context.Context, channel string, presence Presence) error
	Subscribe(ctx context.Context, channel string, handler func(LifecycleEvent)) error
}
