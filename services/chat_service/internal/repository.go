package chat

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, messageDB *MessageDB) (*MessageDB, error)
}
