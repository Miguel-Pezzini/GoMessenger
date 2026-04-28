package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strings"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
)

type Handler struct {
	service       *Service
	publisher     audit.Publisher
	internalToken string
}

type tokenResponse struct {
	Token      string `json:"token"`
	Role       string `json:"role"`
	FriendCode string `json:"friendCode,omitempty"`
}

func NewHandler(service *Service, publisher audit.Publisher, internalToken string) *Handler {
	return &Handler{service: service, publisher: publisher, internalToken: strings.TrimSpace(internalToken)}
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

	writeJSON(w, stdhttp.StatusCreated, tokenResponse{Token: res.Token, Role: res.Role, FriendCode: res.FriendCode})
}

func (h *Handler) BootstrapAdmin(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.internalToken == "" || r.Header.Get("X-Internal-Token") != h.internalToken {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	var req RegisterRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeJSONError(w, stdhttp.StatusBadRequest, "invalid payload")
		return
	}

	res, err := h.service.RegisterAdmin(r.Context(), &req)
	if err != nil {
		h.publish(r.Context(), audit.Event{
			EventType:  "admin.bootstrap.failed",
			Category:   audit.CategoryError,
			Service:    "auth",
			EntityType: "user",
			Status:     audit.StatusFailure,
			Message:    "admin bootstrap failed",
			Metadata: map[string]any{
				"username": req.Username,
				"error":    err.Error(),
			},
		})
		handleError(w, err)
		return
	}

	h.publish(r.Context(), audit.Event{
		EventType:  "admin.bootstrapped",
		Category:   audit.CategoryAudit,
		Service:    "auth",
		EntityType: "user",
		Status:     audit.StatusSuccess,
		Message:    "admin user bootstrapped",
		Metadata: map[string]any{
			"username": req.Username,
		},
	})

	writeJSON(w, stdhttp.StatusCreated, tokenResponse{Token: res.Token, Role: res.Role, FriendCode: res.FriendCode})
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

	writeJSON(w, stdhttp.StatusOK, tokenResponse{Token: res.Token, Role: res.Role, FriendCode: res.FriendCode})
}

func (h *Handler) LookupUserByFriendCode(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.internalToken == "" || r.Header.Get("X-Internal-Token") != h.internalToken {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	code := strings.TrimSpace(r.PathValue("code"))
	userID, err := h.service.LookupUserIDByFriendCode(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			writeJSONError(w, stdhttp.StatusNotFound, "unknown friend code")
		case errors.Is(err, ErrFriendCodeRequired):
			writeJSONError(w, stdhttp.StatusBadRequest, err.Error())
		default:
			writeJSONError(w, stdhttp.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]string{"userId": userID})
}

func (h *Handler) LookupUsernameByUserID(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.internalToken == "" || r.Header.Get("X-Internal-Token") != h.internalToken {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	userID := strings.TrimSpace(r.PathValue("userId"))
	username, err := h.service.GetUsernameByUserID(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			writeJSONError(w, stdhttp.StatusNotFound, "user not found")
		default:
			writeJSONError(w, stdhttp.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, stdhttp.StatusOK, map[string]string{"username": username})
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
