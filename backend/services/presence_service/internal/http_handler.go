package presence

import (
	"encoding/json"
	"errors"
	"net/http"
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
