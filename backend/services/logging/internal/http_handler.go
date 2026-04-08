package logging

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/security"
	"github.com/gorilla/websocket"
)

type Handler struct {
	service         *Service
	originValidator security.OriginValidator
}

func NewHandler(service *Service, originValidator security.OriginValidator) *Handler {
	return &Handler{service: service, originValidator: originValidator}
}

func (h *Handler) ListLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	events, err := h.service.ListRecent(r.Context(), limit)
	if err != nil {
		http.Error(w, "failed to list logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.originValidator.IsAllowed,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch, unsubscribe := h.service.Subscribe()
	defer unsubscribe()

	for event := range ch {
		if err := conn.WriteJSON(event); err != nil {
			return
		}
	}
}
