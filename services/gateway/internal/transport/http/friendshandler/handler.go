package friendshandler

import (
	"encoding/json"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/domain/friends"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req friends.SendFriendRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	friendRequest, err := h.service.SendFriendRequest(r.Context(), actorID, req)
	if err != nil {
		handleGrpcError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, friendRequest)
}

func (h *Handler) AcceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.AcceptFriendRequest(r.Context(), actorID, r.PathValue("id")); err != nil {
		handleGrpcError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"accepted": true})
}

func (h *Handler) DeclineFriendRequest(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.DeclineFriendRequest(r.Context(), actorID, r.PathValue("id")); err != nil {
		handleGrpcError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.RemoveFriend(r.Context(), actorID, r.PathValue("friendId")); err != nil {
		handleGrpcError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListFriends(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	friendsList, err := h.service.ListFriends(r.Context(), actorID)
	if err != nil {
		handleGrpcError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, friendsList)
}

func (h *Handler) ListPendingFriendRequests(w http.ResponseWriter, r *http.Request) {
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	requests, err := h.service.ListPendingFriendRequests(r.Context(), actorID)
	if err != nil {
		handleGrpcError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, requests)
}

func actorIDFromContext(r *http.Request) (string, bool) {
	actorID, ok := r.Context().Value(middleware.UserIDKey).(string)
	return actorID, ok && actorID != ""
}

func handleGrpcError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	switch st.Code() {
	case codes.InvalidArgument:
		writeJSONError(w, http.StatusBadRequest, st.Message())
	case codes.NotFound:
		writeJSONError(w, http.StatusNotFound, st.Message())
	case codes.AlreadyExists:
		writeJSONError(w, http.StatusConflict, st.Message())
	case codes.PermissionDenied:
		writeJSONError(w, http.StatusForbidden, st.Message())
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
