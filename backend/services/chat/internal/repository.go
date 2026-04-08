package chat

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, messageDB *MessageDB) (*MessageDB, bool, error)
	GetConversation(ctx context.Context, userA, userB, before string, limit int) ([]MessageDB, error)
	UpdateViewedStatus(ctx context.Context, messageID, receiverUserID, status string) (*MessageDB, error)
}
