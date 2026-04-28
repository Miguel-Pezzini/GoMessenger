package media

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"
)

const (
	userIDHeader        = "X-User-ID"
	internalTokenHeader = "X-Internal-Token"
)

type Handler struct {
	service       *Service
	internalToken string
}

func NewHandler(service *Service, internalToken string) *Handler {
	return &Handler{service: service, internalToken: internalToken}
}

func (h *Handler) UploadAttachments(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxFilesPerUpload*MaxFileSizeBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart upload")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	files := r.MultipartForm.File["files"]
	attachments, err := h.service.UploadFiles(r.Context(), userID, files)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, UploadResponse{Attachments: attachments})
}

func (h *Handler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(userIDHeader)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	attachment, object, err := h.service.OpenAttachment(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer object.Body.Close()

	contentType := attachment.ContentType
	if contentType == "" {
		contentType = object.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition(*attachment))
	if attachment.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(attachment.Size, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = copyResponse(w, object.Body)
}

func (h *Handler) PrepareMessage(w http.ResponseWriter, r *http.Request) {
	if !h.authorizedInternal(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req PrepareMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	attachments, err := h.service.PrepareMessage(r.Context(), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, PrepareMessageResponse{Attachments: attachments})
}

func (h *Handler) BindMessage(w http.ResponseWriter, r *http.Request) {
	if !h.authorizedInternal(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req BindMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	attachments, err := h.service.BindMessage(r.Context(), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, BindMessageResponse{Attachments: attachments})
}

func (h *Handler) CleanupExpired(w http.ResponseWriter, r *http.Request) {
	if !h.authorizedInternal(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}

	deleted, err := h.service.CleanupExpired(r.Context(), time.Now().UTC(), limit)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, CleanupExpiredResponse{Deleted: deleted})
}

func (h *Handler) authorizedInternal(r *http.Request) bool {
	return h.internalToken != "" && r.Header.Get(internalTokenHeader) == h.internalToken
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}

func contentDisposition(attachment Attachment) string {
	disposition := "attachment"
	if attachment.Kind == KindImage || attachment.Kind == KindVideo || attachment.Kind == KindAudio || attachment.ContentType == "application/pdf" {
		disposition = "inline"
	}
	return mime.FormatMediaType(disposition, map[string]string{"filename": attachment.Filename})
}

func copyResponse(w http.ResponseWriter, body interface {
	Read([]byte) (int, error)
}) (int64, error) {
	return io.Copy(w, body)
}
