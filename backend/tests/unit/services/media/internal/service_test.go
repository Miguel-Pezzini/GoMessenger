package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"
)

type repositoryStub struct {
	attachments map[string]Attachment
}

func newRepositoryStub() *repositoryStub {
	return &repositoryStub{attachments: map[string]Attachment{}}
}

func (r *repositoryStub) Create(_ context.Context, attachment *Attachment) error {
	r.attachments[attachment.ID] = *attachment
	return nil
}

func (r *repositoryStub) FindByID(_ context.Context, id string) (*Attachment, error) {
	attachment, ok := r.attachments[id]
	if !ok || attachment.Status == StatusDeleted {
		return nil, ErrNotFound
	}
	return &attachment, nil
}

func (r *repositoryStub) FindByIDs(_ context.Context, ids []string) ([]Attachment, error) {
	attachments := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		attachment, ok := r.attachments[id]
		if !ok || attachment.Status == StatusDeleted {
			continue
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func (r *repositoryStub) Bind(_ context.Context, ids []string, messageID, senderID, receiverID string) ([]Attachment, error) {
	attachments := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		attachment, ok := r.attachments[id]
		if !ok {
			return nil, ErrNotFound
		}
		if attachment.OwnerID != senderID {
			return nil, ErrForbidden
		}
		if attachment.Status == StatusBound && attachment.BoundMessageID != messageID {
			return nil, ErrForbidden
		}
		if attachment.Status != StatusUploaded && attachment.Status != StatusBound {
			return nil, ErrForbidden
		}
		attachment.Status = StatusBound
		attachment.BoundMessageID = messageID
		attachment.AuthorizedUserIDs = []string{senderID, receiverID}
		attachment.ExpiresAt = nil
		r.attachments[id] = attachment
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func (r *repositoryStub) ListExpired(_ context.Context, now time.Time, _ int) ([]Attachment, error) {
	var attachments []Attachment
	for _, attachment := range r.attachments {
		if attachment.Status == StatusUploaded && attachment.ExpiresAt != nil && !attachment.ExpiresAt.After(now) {
			attachments = append(attachments, attachment)
		}
	}
	return attachments, nil
}

func (r *repositoryStub) MarkDeleted(_ context.Context, id string) error {
	attachment, ok := r.attachments[id]
	if !ok {
		return ErrNotFound
	}
	attachment.Status = StatusDeleted
	r.attachments[id] = attachment
	return nil
}

type storageStub struct {
	objects map[string][]byte
	putErr  error
}

func newStorageStub() *storageStub {
	return &storageStub{objects: map[string][]byte{}}
}

func (s *storageStub) EnsureBucket(_ context.Context) error {
	return nil
}

func (s *storageStub) PutObject(_ context.Context, key, _ string, _ int64, body interface {
	Read([]byte) (int, error)
}) error {
	if s.putErr != nil {
		return s.putErr
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[key] = payload
	return nil
}

func (s *storageStub) GetObject(_ context.Context, key string) (*StoredObject, error) {
	payload, ok := s.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return &StoredObject{
		Body:          io.NopCloser(bytes.NewReader(payload)),
		ContentLength: int64(len(payload)),
		ContentType:   "application/octet-stream",
	}, nil
}

func (s *storageStub) DeleteObject(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func TestUploadFilesStoresObjectAndMetadata(t *testing.T) {
	repo := newRepositoryStub()
	storage := newStorageStub()
	service := NewService(repo, storage, time.Hour)

	files := multipartFiles(t, "photo.jpg", "image/jpeg", []byte("image-bytes"))
	attachments, err := service.UploadFiles(context.Background(), "user-a", files)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(attachments))
	}
	stored := repo.attachments[attachments[0].ID]
	if stored.OwnerID != "user-a" || stored.Status != StatusUploaded {
		t.Fatalf("unexpected stored attachment: %+v", stored)
	}
	if stored.Kind != KindImage {
		t.Fatalf("expected image kind, got %s", stored.Kind)
	}
	if string(storage.objects[stored.ObjectKey]) != "image-bytes" {
		t.Fatalf("expected object bytes to be stored")
	}
}

func TestUploadFilesRejectsOversizedFile(t *testing.T) {
	service := NewService(newRepositoryStub(), newStorageStub(), time.Hour)

	_, err := service.UploadFiles(context.Background(), "user-a", []*multipart.FileHeader{{
		Filename: "huge.bin",
		Size:     MaxFileSizeBytes + 1,
	}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestPrepareMessageRequiresSenderOwnedUploadedAttachments(t *testing.T) {
	repo := newRepositoryStub()
	service := NewService(repo, newStorageStub(), time.Hour)
	repo.attachments["attachment-1"] = Attachment{
		ID:       "attachment-1",
		OwnerID:  "user-a",
		Status:   StatusUploaded,
		Filename: "photo.jpg",
		Kind:     KindImage,
	}

	attachments, err := service.PrepareMessage(context.Background(), PrepareMessageRequest{
		SenderID:      "user-a",
		ReceiverID:    "user-b",
		AttachmentIDs: []string{"attachment-1"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(attachments) != 1 || attachments[0].ID != "attachment-1" {
		t.Fatalf("expected prepared attachment, got %+v", attachments)
	}

	_, err = service.PrepareMessage(context.Background(), PrepareMessageRequest{
		SenderID:      "user-other",
		ReceiverID:    "user-b",
		AttachmentIDs: []string{"attachment-1"},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestBindMessageGrantsReceiverAccess(t *testing.T) {
	repo := newRepositoryStub()
	service := NewService(repo, newStorageStub(), time.Hour)
	repo.attachments["attachment-1"] = Attachment{
		ID:       "attachment-1",
		OwnerID:  "user-a",
		Status:   StatusUploaded,
		Filename: "photo.jpg",
	}

	_, err := service.BindMessage(context.Background(), BindMessageRequest{
		MessageID:     "message-1",
		SenderID:      "user-a",
		ReceiverID:    "user-b",
		AttachmentIDs: []string{"attachment-1"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	stored := repo.attachments["attachment-1"]
	if stored.Status != StatusBound || stored.BoundMessageID != "message-1" {
		t.Fatalf("expected bound attachment, got %+v", stored)
	}
	if !attachmentAllowsUser(stored, "user-b") {
		t.Fatalf("expected receiver to be authorized")
	}
}

func TestOpenAttachmentRequiresAuthorizedUser(t *testing.T) {
	repo := newRepositoryStub()
	storage := newStorageStub()
	service := NewService(repo, storage, time.Hour)
	repo.attachments["attachment-1"] = Attachment{
		ID:                "attachment-1",
		OwnerID:           "user-a",
		ObjectKey:         "attachments/attachment-1/blob",
		Status:            StatusBound,
		AuthorizedUserIDs: []string{"user-a", "user-b"},
	}
	storage.objects["attachments/attachment-1/blob"] = []byte("hello")

	_, object, err := service.OpenAttachment(context.Background(), "user-b", "attachment-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	object.Body.Close()

	_, _, err = service.OpenAttachment(context.Background(), "user-c", "attachment-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestCleanupExpiredDeletesObjectsAndMarksMetadataDeleted(t *testing.T) {
	repo := newRepositoryStub()
	storage := newStorageStub()
	service := NewService(repo, storage, time.Hour)
	now := time.Now().UTC()
	expiresAt := now.Add(-time.Minute)
	repo.attachments["attachment-1"] = Attachment{
		ID:        "attachment-1",
		ObjectKey: "attachments/attachment-1/blob",
		Status:    StatusUploaded,
		ExpiresAt: &expiresAt,
	}
	storage.objects["attachments/attachment-1/blob"] = []byte("hello")

	deleted, err := service.CleanupExpired(context.Background(), now, 100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected one deleted attachment, got %d", deleted)
	}
	if _, ok := storage.objects["attachments/attachment-1/blob"]; ok {
		t.Fatalf("expected object to be deleted")
	}
	if repo.attachments["attachment-1"].Status != StatusDeleted {
		t.Fatalf("expected metadata to be marked deleted")
	}
}

func multipartFiles(t *testing.T, filename, contentType string, payload []byte) []*multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="files"; filename="`+filename+`"`)
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("failed to write multipart part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/attachments", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}
	return req.MultipartForm.File["files"]
}
