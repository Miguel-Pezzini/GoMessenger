package presence

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) HandleGetPresence(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	if userID == "" {
		http.Error(w, "missing user id", http.StatusBadRequest)
		return
	}

	presence, err := h.service.GetPresence(r.Context(), userID)
	if errors.Is(err, ErrPresenceNotFound) {
		http.Error(w, "presence not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load presence", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(presence)
}

func (h *Handler) HandleListActiveUsers(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	active, err := h.service.ListActiveUsers(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to load active users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(active)
}
