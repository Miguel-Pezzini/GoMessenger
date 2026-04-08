package chat

import (
	"encoding/json"
	"net/http"
	"strconv"
)

const userIDHeader = "X-User-ID"

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetConversation handles GET /messages/{userId}
// Query params:
//
//	before=<message_id>  — cursor: load messages older than this ID
//	limit=<n>            — default 20, max 100
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	callerID := r.Header.Get(userIDHeader)
	if callerID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	otherUserID := r.PathValue("userId")
	if otherUserID == "" {
		writeError(w, http.StatusBadRequest, "userId path param required")
		return
	}

	before := r.URL.Query().Get("before")

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}

	result, err := h.service.GetConversation(r.Context(), callerID, otherUserID, before, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
