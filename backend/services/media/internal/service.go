package media

import (
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	MaxFilesPerUpload       = 10
	MaxFileSizeBytes  int64 = 25 << 20

	defaultUploadTTL = time.Hour
)

type Service struct {
	repo      Repository
	storage   Storage
	uploadTTL time.Duration
}

type uploadedFileRecord struct {
	objectKey    string
	attachmentID string
	created      bool
}

func NewService(repo Repository, storage Storage, uploadTTL time.Duration) *Service {
	if uploadTTL <= 0 {
		uploadTTL = defaultUploadTTL
	}
	return &Service{repo: repo, storage: storage, uploadTTL: uploadTTL}
}

func (s *Service) UploadFiles(ctx context.Context, ownerID string, files []*multipart.FileHeader) ([]AttachmentSnapshot, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("%w: owner_id is required", ErrInvalidInput)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: at least one file is required", ErrInvalidInput)
	}
	if len(files) > MaxFilesPerUpload {
		return nil, fmt.Errorf("%w: at most %d files are allowed", ErrInvalidInput, MaxFilesPerUpload)
	}

	for _, header := range files {
		if header == nil {
			return nil, fmt.Errorf("%w: file is required", ErrInvalidInput)
		}
		if header.Size <= 0 {
			return nil, fmt.Errorf("%w: file must not be empty", ErrInvalidInput)
		}
		if header.Size > MaxFileSizeBytes {
			return nil, fmt.Errorf("%w: file exceeds %d bytes", ErrInvalidInput, MaxFileSizeBytes)
		}
	}

	snapshots := make([]AttachmentSnapshot, 0, len(files))
	now := time.Now().UTC()
	expiresAt := now.Add(s.uploadTTL)
	uploaded := make([]uploadedFileRecord, 0, len(files))

	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			s.rollbackUploadedFiles(ctx, uploaded)
			return nil, err
		}

		id := primitive.NewObjectID().Hex()
		filename := sanitizeFilename(header.Filename)
		contentType := detectContentType(header)
		attachment := Attachment{
			ID:                id,
			OwnerID:           ownerID,
			ObjectKey:         objectKeyFor(id, filename),
			Filename:          filename,
			ContentType:       contentType,
			Size:              header.Size,
			Kind:              kindForContentType(contentType),
			Status:            StatusUploaded,
			AuthorizedUserIDs: []string{ownerID},
			CreatedAt:         now,
			ExpiresAt:         &expiresAt,
		}

		if err := s.storage.PutObject(ctx, attachment.ObjectKey, attachment.ContentType, attachment.Size, file); err != nil {
			_ = file.Close()
			s.rollbackUploadedFiles(ctx, uploaded)
			return nil, err
		}
		if err := file.Close(); err != nil {
			s.rollbackUploadedFiles(ctx, uploaded)
			return nil, err
		}
		uploaded = append(uploaded, uploadedFileRecord{
			objectKey:    attachment.ObjectKey,
			attachmentID: attachment.ID,
		})

		if err := s.repo.Create(ctx, &attachment); err != nil {
			_ = s.storage.DeleteObject(ctx, attachment.ObjectKey)
			s.rollbackUploadedFiles(ctx, uploaded)
			return nil, err
		}
		uploaded[len(uploaded)-1].created = true

		snapshots = append(snapshots, AttachmentSnapshotFromAttachment(attachment))
	}

	return snapshots, nil
}

func (s *Service) rollbackUploadedFiles(ctx context.Context, uploaded []uploadedFileRecord) {
	for i := len(uploaded) - 1; i >= 0; i-- {
		item := uploaded[i]
		_ = s.storage.DeleteObject(ctx, item.objectKey)
		if item.created {
			_ = s.repo.MarkDeleted(ctx, item.attachmentID)
		}
	}
}

func (s *Service) PrepareMessage(ctx context.Context, req PrepareMessageRequest) ([]AttachmentSnapshot, error) {
	ids, err := validateAttachmentIDs(req.AttachmentIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []AttachmentSnapshot{}, nil
	}

	senderID := strings.TrimSpace(req.SenderID)
	if senderID == "" {
		return nil, fmt.Errorf("%w: sender_id is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.ReceiverID) == "" {
		return nil, fmt.Errorf("%w: receiver_id is required", ErrInvalidInput)
	}

	attachments, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(attachments) != len(ids) {
		return nil, ErrNotFound
	}

	byID := make(map[string]Attachment, len(attachments))
	for _, attachment := range attachments {
		if attachment.OwnerID != senderID || attachment.Status != StatusUploaded {
			return nil, ErrForbidden
		}
		byID[attachment.ID] = attachment
	}

	snapshots := make([]AttachmentSnapshot, 0, len(ids))
	for _, id := range ids {
		attachment, ok := byID[id]
		if !ok {
			return nil, ErrNotFound
		}
		snapshots = append(snapshots, AttachmentSnapshotFromAttachment(attachment))
	}
	return snapshots, nil
}

func (s *Service) BindMessage(ctx context.Context, req BindMessageRequest) ([]AttachmentSnapshot, error) {
	ids, err := validateAttachmentIDs(req.AttachmentIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []AttachmentSnapshot{}, nil
	}
	if strings.TrimSpace(req.MessageID) == "" {
		return nil, fmt.Errorf("%w: message_id is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.SenderID) == "" {
		return nil, fmt.Errorf("%w: sender_id is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.ReceiverID) == "" {
		return nil, fmt.Errorf("%w: receiver_id is required", ErrInvalidInput)
	}

	attachments, err := s.repo.Bind(ctx, ids, req.MessageID, req.SenderID, req.ReceiverID)
	if err != nil {
		return nil, err
	}

	snapshots := make([]AttachmentSnapshot, len(attachments))
	for i, attachment := range attachments {
		snapshots[i] = AttachmentSnapshotFromAttachment(attachment)
	}
	return snapshots, nil
}

func (s *Service) OpenAttachment(ctx context.Context, userID, attachmentID string) (*Attachment, *StoredObject, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil, fmt.Errorf("%w: user_id is required", ErrInvalidInput)
	}

	attachment, err := s.repo.FindByID(ctx, strings.TrimSpace(attachmentID))
	if err != nil {
		return nil, nil, err
	}
	if !attachmentAllowsUser(*attachment, userID) {
		return nil, nil, ErrForbidden
	}

	object, err := s.storage.GetObject(ctx, attachment.ObjectKey)
	if err != nil {
		return nil, nil, err
	}
	return attachment, object, nil
}

func (s *Service) CleanupExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	attachments, err := s.repo.ListExpired(ctx, now, limit)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, attachment := range attachments {
		if err := s.storage.DeleteObject(ctx, attachment.ObjectKey); err != nil {
			return deleted, err
		}
		if err := s.repo.MarkDeleted(ctx, attachment.ID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func validateAttachmentIDs(ids []string) ([]string, error) {
	if len(ids) > MaxFilesPerUpload {
		return nil, fmt.Errorf("%w: at most %d attachments are allowed", ErrInvalidInput, MaxFilesPerUpload)
	}

	seen := map[string]bool{}
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("%w: attachment_id is required", ErrInvalidInput)
		}
		if seen[id] {
			return nil, fmt.Errorf("%w: duplicate attachment_id", ErrInvalidInput)
		}
		seen[id] = true
		clean = append(clean, id)
	}
	return clean, nil
}

func sanitizeFilename(filename string) string {
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		return "file"
	}
	return filename
}

func objectKeyFor(id, _ string) string {
	return "attachments/" + id + "/blob"
}

func detectContentType(header *multipart.FileHeader) string {
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType != "" {
		return contentType
	}
	if extType := mime.TypeByExtension(filepath.Ext(header.Filename)); extType != "" {
		if base, _, err := mime.ParseMediaType(extType); err == nil {
			return base
		}
		return extType
	}
	return "application/octet-stream"
}

func kindForContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return KindImage
	case strings.HasPrefix(contentType, "video/"):
		return KindVideo
	case strings.HasPrefix(contentType, "audio/"):
		return KindAudio
	case contentType == "application/pdf" || strings.HasPrefix(contentType, "text/"):
		return KindDocument
	case strings.Contains(contentType, "word") || strings.Contains(contentType, "spreadsheet") || strings.Contains(contentType, "presentation"):
		return KindDocument
	default:
		return KindFile
	}
}

func attachmentAllowsUser(attachment Attachment, userID string) bool {
	if attachment.OwnerID == userID {
		return true
	}
	for _, authorized := range attachment.AuthorizedUserIDs {
		if authorized == userID {
			return true
		}
	}
	return false
}
