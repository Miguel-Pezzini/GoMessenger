package media

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, attachment *Attachment) error
	FindByID(ctx context.Context, id string) (*Attachment, error)
	FindByIDs(ctx context.Context, ids []string) ([]Attachment, error)
	Bind(ctx context.Context, ids []string, messageID, senderID, receiverID string) ([]Attachment, error)
	ListExpired(ctx context.Context, now time.Time, limit int) ([]Attachment, error)
	MarkDeleted(ctx context.Context, id string) error
}

type StoredObject struct {
	Body interface {
		Read([]byte) (int, error)
		Close() error
	}
	ContentLength int64
	ContentType   string
}

type Storage interface {
	EnsureBucket(ctx context.Context) error
	PutObject(ctx context.Context, key, contentType string, size int64, body interface {
		Read([]byte) (int, error)
	}) error
	GetObject(ctx context.Context, key string) (*StoredObject, error)
	DeleteObject(ctx context.Context, key string) error
}
