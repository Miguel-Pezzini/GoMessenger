package media

import (
	"errors"
	"time"
)

const (
	StatusUploaded = "uploaded"
	StatusBound    = "bound"
	StatusDeleted  = "deleted"

	KindImage    = "image"
	KindVideo    = "video"
	KindAudio    = "audio"
	KindDocument = "document"
	KindFile     = "file"
)

var (
	ErrNotFound     = errors.New("attachment not found")
	ErrForbidden    = errors.New("attachment forbidden")
	ErrInvalidInput = errors.New("invalid attachment input")
)

type Attachment struct {
	ID                string
	OwnerID           string
	ObjectKey         string
	Filename          string
	ContentType       string
	Size              int64
	Kind              string
	Status            string
	BoundMessageID    string
	AuthorizedUserIDs []string
	CreatedAt         time.Time
	BoundAt           time.Time
	ExpiresAt         *time.Time
}

type AttachmentSnapshot struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Kind        string `json:"kind"`
	DownloadURL string `json:"download_url"`
}

type UploadResponse struct {
	Attachments []AttachmentSnapshot `json:"attachments"`
}

type PrepareMessageRequest struct {
	SenderID      string   `json:"sender_id"`
	ReceiverID    string   `json:"receiver_id"`
	AttachmentIDs []string `json:"attachment_ids"`
}

type PrepareMessageResponse struct {
	Attachments []AttachmentSnapshot `json:"attachments"`
}

type BindMessageRequest struct {
	MessageID     string   `json:"message_id"`
	SenderID      string   `json:"sender_id"`
	ReceiverID    string   `json:"receiver_id"`
	AttachmentIDs []string `json:"attachment_ids"`
}

type BindMessageResponse struct {
	Attachments []AttachmentSnapshot `json:"attachments"`
}

type CleanupExpiredResponse struct {
	Deleted int `json:"deleted"`
}

func AttachmentSnapshotFromAttachment(attachment Attachment) AttachmentSnapshot {
	return AttachmentSnapshot{
		ID:          attachment.ID,
		Filename:    attachment.Filename,
		ContentType: attachment.ContentType,
		Size:        attachment.Size,
		Kind:        attachment.Kind,
		DownloadURL: "/attachments/" + attachment.ID,
	}
}
