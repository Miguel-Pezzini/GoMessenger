package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
)

type Handler struct {
	service   *Service
	publisher audit.Publisher
}

type authResponse struct {
	Token string `json:"token"`
}

func NewHandler(service *Service, publisher audit.Publisher) *Handler {
	return &Handler{service: service, publisher: publisher}
}

func (h *Handler) Register(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req RegisterRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeJSONError(w, stdhttp.StatusBadRequest, "invalid payload")
		return
	}

	res, err := h.service.Register(r.Context(), &req)
	if err != nil {
		h.publish(r.Context(), audit.Event{
			EventType:  "user.register.failed",
			Category:   audit.CategoryError,
			Service:    "auth",
			EntityType: "user",
			Status:     audit.StatusFailure,
			Message:    "user registration failed",
			Metadata: map[string]any{
				"username": req.Username,
				"error":    err.Error(),
			},
		})
		handleError(w, err)
		return
	}

	h.publish(r.Context(), audit.Event{
		EventType:  "user.registered",
		Category:   audit.CategoryAudit,
		Service:    "auth",
		EntityType: "user",
		Status:     audit.StatusSuccess,
		Message:    "user registered",
		Metadata: map[string]any{
			"username": req.Username,
		},
	})

	writeJSON(w, stdhttp.StatusCreated, authResponse{Token: res.Token})
}

func (h *Handler) Login(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	var req LoginRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeJSONError(w, stdhttp.StatusBadRequest, "invalid payload")
		return
	}

	res, err := h.service.Authenticate(r.Context(), &req)
	if err != nil {
		h.publish(r.Context(), audit.Event{
			EventType:  "user.login.failed",
			Category:   audit.CategoryError,
			Service:    "auth",
			EntityType: "session",
			Status:     audit.StatusFailure,
			Message:    "user login failed",
			Metadata: map[string]any{
				"username": req.Username,
				"error":    err.Error(),
			},
		})
		handleError(w, err)
		return
	}

	h.publish(r.Context(), audit.Event{
		EventType:  "user.logged_in",
		Category:   audit.CategoryAudit,
		Service:    "auth",
		EntityType: "session",
		Status:     audit.StatusSuccess,
		Message:    "user logged in",
		Metadata: map[string]any{
			"username": req.Username,
		},
	})

	writeJSON(w, stdhttp.StatusOK, authResponse{Token: res.Token})
}

func (h *Handler) publish(ctx context.Context, event audit.Event) {
	if h.publisher == nil {
		return
	}
	_ = h.publisher.Publish(ctx, event)
}

func handleError(w stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrInvalidPassword):
		writeJSONError(w, stdhttp.StatusBadRequest, err.Error())
	case errors.Is(err, ErrUserAlreadyExists):
		writeJSONError(w, stdhttp.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		writeJSONError(w, stdhttp.StatusUnauthorized, err.Error())
	default:
		writeJSONError(w, stdhttp.StatusInternalServerError, "internal server error")
	}
}

func decodeJSONBody(w stdhttp.ResponseWriter, r *stdhttp.Request, dst any) error {
	decoder := json.NewDecoder(stdhttp.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty body")
		}
		return err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple json values")
		}
		return err
	}

	return nil
}

func writeJSONError(w stdhttp.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func writeJSON(w stdhttp.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
