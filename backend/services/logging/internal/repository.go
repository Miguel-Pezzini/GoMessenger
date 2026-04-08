package logging

import "context"

type Repository interface {
	Append(ctx context.Context, event StoredEvent) error
	ListRecent(ctx context.Context, limit int) ([]StoredEvent, error)
}
