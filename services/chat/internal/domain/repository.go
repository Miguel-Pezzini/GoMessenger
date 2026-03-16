package domain

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, messageDB *MessageDB) (*MessageDB, error)
}
