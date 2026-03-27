package friendshandler

import (
	"encoding/json"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/domain/friends"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/middleware"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/response"
)

type Handler struct {
	service *friends.Service
}

func New(service *friends.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SendFriendRequest(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req friends.SendFriendRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	friendRequest, err := h.service.SendFriendRequest(r.Context(), actorID, req)
	if err != nil {
		response.HandleGRPCError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, friendRequest)
}

func (h *Handler) AcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.AcceptFriendRequest(r.Context(), actorID, r.PathValue("id")); err != nil {
		response.HandleGRPCError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]bool{"accepted": true})
}

func (h *Handler) DeclineFriendRequest(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.DeclineFriendRequest(r.Context(), actorID, r.PathValue("id")); err != nil {
		response.HandleGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.RemoveFriend(r.Context(), actorID, r.PathValue("friendId")); err != nil {
		response.HandleGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListFriends(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	friendsList, err := h.service.ListFriends(r.Context(), actorID)
	if err != nil {
		response.HandleGRPCError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, friendsList)
}

func (h *Handler) ListPendingFriendRequests(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requests, err := h.service.ListPendingFriendRequests(r.Context(), actorID)
	if err != nil {
		response.HandleGRPCError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, requests)
}

func actorIDFromContext(r *http.Request) (string, bool) {
	actorID, ok := r.Context().Value(middleware.UserIDKey).(string)
	return actorID, ok && actorID != ""
}
