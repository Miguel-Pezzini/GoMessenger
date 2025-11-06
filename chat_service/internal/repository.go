package chat

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, message Message) (*Message, error)
}
