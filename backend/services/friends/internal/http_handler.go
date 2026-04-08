package friends

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
)

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

type EventPublisher interface {
	Publish(channel string, payload any)
}

type NotificationPublisher interface {
	PublishFriendRequestNotificationIntent(senderID, receiverID, friendRequestID string) error
}

type Handler struct {
	service               *Service
	publisher             EventPublisher
	notificationPublisher NotificationPublisher
	auditPublisher        audit.Publisher
	channel               string
}

type sendFriendRequestRequest struct {
	ReceiverID string `json:"receiverId"`
}

type friendResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	FriendID  string `json:"friendId"`
	CreatedAt string `json:"createdAt"`
}

type friendRequestResponse struct {
	ID         string `json:"id"`
	SenderID   string `json:"senderId"`
	ReceiverID string `json:"receiverId"`
	CreatedAt  string `json:"createdAt"`
}

type friendEvent struct {
	TargetUserID string          `json:"target_user_id"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
}

func NewHandler(service *Service, publisher EventPublisher, notificationPublisher NotificationPublisher, auditPublisher audit.Publisher, channel string) *Handler {
	return &Handler{service: service, publisher: publisher, notificationPublisher: notificationPublisher, auditPublisher: auditPublisher, channel: channel}
}

func (h *Handler) SendFriendRequest(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := actorIDFromHeader(r)
	if !ok {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	var req sendFriendRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, stdhttp.StatusBadRequest, "invalid payload")
		return
	}

	request, err := h.service.SendFriendRequest(r.Context(), actorID, req.ReceiverID)
	if err != nil {
		h.publishAudit(r.Context(), audit.Event{
			EventType:    "friend_request.create.failed",
			Category:     audit.CategoryError,
			Service:      "friends",
			ActorUserID:  actorID,
			TargetUserID: req.ReceiverID,
			EntityType:   "friend_request",
			Status:       audit.StatusFailure,
			Message:      "friend request creation failed",
			Metadata:     map[string]any{"error": err.Error()},
		})
		handleError(w, err)
		return
	}

	h.publish(request.ReceiverID, "friend_request_received", map[string]string{
		"id":        request.ID,
		"sender_id": request.SenderID,
	})
	if h.notificationPublisher != nil {
		if err := h.notificationPublisher.PublishFriendRequestNotificationIntent(request.SenderID, request.ReceiverID, request.ID); err != nil {
			h.publishAudit(r.Context(), audit.Event{
				EventType:    "friend_request.notification.failed",
				Category:     audit.CategoryError,
				Service:      "friends",
				ActorUserID:  actorID,
				TargetUserID: request.ReceiverID,
				EntityType:   "friend_request",
				EntityID:     request.ID,
				Status:       audit.StatusFailure,
				Message:      "friend request notification enqueue failed",
				Metadata:     map[string]any{"error": err.Error()},
			})
		}
	}
	h.publishAudit(r.Context(), audit.Event{
		EventType:    "friend_request.created",
		Category:     audit.CategoryAudit,
		Service:      "friends",
		ActorUserID:  actorID,
		TargetUserID: request.ReceiverID,
		EntityType:   "friend_request",
		EntityID:     request.ID,
		Status:       audit.StatusSuccess,
		Message:      "friend request created",
	})

	writeJSON(w, stdhttp.StatusCreated, mapFriendRequest(request))
}

func (h *Handler) AcceptFriendRequest(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := actorIDFromHeader(r)
	if !ok {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	request, err := h.service.AcceptFriendRequest(r.Context(), actorID, r.PathValue("id"))
	if err != nil {
		h.publishAudit(r.Context(), audit.Event{
			EventType:   "friend_request.accept.failed",
			Category:    audit.CategoryError,
			Service:     "friends",
			ActorUserID: actorID,
			EntityType:  "friend_request",
			EntityID:    r.PathValue("id"),
			Status:      audit.StatusFailure,
			Message:     "friend request acceptance failed",
			Metadata:    map[string]any{"error": err.Error()},
		})
		handleError(w, err)
		return
	}

	h.publish(request.SenderID, "friend_request_accepted", map[string]string{
		"id":          request.ID,
		"acceptor_id": actorID,
	})
	h.publishAudit(r.Context(), audit.Event{
		EventType:    "friend_request.accepted",
		Category:     audit.CategoryAudit,
		Service:      "friends",
		ActorUserID:  actorID,
		TargetUserID: request.SenderID,
		EntityType:   "friend_request",
		EntityID:     request.ID,
		Status:       audit.StatusSuccess,
		Message:      "friend request accepted",
	})

	writeJSON(w, stdhttp.StatusOK, map[string]bool{"accepted": true})
}

func (h *Handler) DeclineFriendRequest(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := actorIDFromHeader(r)
	if !ok {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	request, err := h.service.DeclineFriendRequest(r.Context(), actorID, r.PathValue("id"))
	if err != nil {
		h.publishAudit(r.Context(), audit.Event{
			EventType:   "friend_request.decline.failed",
			Category:    audit.CategoryError,
			Service:     "friends",
			ActorUserID: actorID,
			EntityType:  "friend_request",
			EntityID:    r.PathValue("id"),
			Status:      audit.StatusFailure,
			Message:     "friend request decline failed",
			Metadata:    map[string]any{"error": err.Error()},
		})
		handleError(w, err)
		return
	}

	h.publish(request.SenderID, "friend_request_declined", map[string]string{
		"id":          request.ID,
		"decliner_id": actorID,
	})
	h.publishAudit(r.Context(), audit.Event{
		EventType:    "friend_request.declined",
		Category:     audit.CategoryAudit,
		Service:      "friends",
		ActorUserID:  actorID,
		TargetUserID: request.SenderID,
		EntityType:   "friend_request",
		EntityID:     request.ID,
		Status:       audit.StatusSuccess,
		Message:      "friend request declined",
	})

	w.WriteHeader(stdhttp.StatusNoContent)
}

func (h *Handler) RemoveFriend(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := actorIDFromHeader(r)
	if !ok {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	friendID := r.PathValue("friendId")
	if err := h.service.RemoveFriend(r.Context(), actorID, friendID); err != nil {
		h.publishAudit(r.Context(), audit.Event{
			EventType:    "friendship.remove.failed",
			Category:     audit.CategoryError,
			Service:      "friends",
			ActorUserID:  actorID,
			TargetUserID: friendID,
			EntityType:   "friendship",
			Status:       audit.StatusFailure,
			Message:      "friend removal failed",
			Metadata:     map[string]any{"error": err.Error()},
		})
		handleError(w, err)
		return
	}

	h.publish(friendID, "friend_removed", map[string]string{
		"remover_id": actorID,
	})
	h.publishAudit(r.Context(), audit.Event{
		EventType:    "friendship.removed",
		Category:     audit.CategoryAudit,
		Service:      "friends",
		ActorUserID:  actorID,
		TargetUserID: friendID,
		EntityType:   "friendship",
		Status:       audit.StatusSuccess,
		Message:      "friend removed",
	})

	w.WriteHeader(stdhttp.StatusNoContent)
}

func (h *Handler) ListFriends(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := actorIDFromHeader(r)
	if !ok {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	friendsList, err := h.service.ListFriends(r.Context(), actorID)
	if err != nil {
		handleError(w, err)
		return
	}

	response := make([]friendResponse, 0, len(friendsList))
	for _, friend := range friendsList {
		response = append(response, mapFriend(friend))
	}
	writeJSON(w, stdhttp.StatusOK, response)
}

func (h *Handler) ListPendingFriendRequests(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	actorID, ok := actorIDFromHeader(r)
	if !ok {
		writeJSONError(w, stdhttp.StatusUnauthorized, "unauthorized")
		return
	}

	requests, err := h.service.ListPendingFriendRequests(r.Context(), actorID)
	if err != nil {
		handleError(w, err)
		return
	}

	response := make([]friendRequestResponse, 0, len(requests))
	for _, request := range requests {
		response = append(response, mapFriendRequest(request))
	}
	writeJSON(w, stdhttp.StatusOK, response)
}

func (h *Handler) publish(targetUserID, eventType string, payload any) {
	if h.publisher == nil {
		return
	}

	data, _ := json.Marshal(payload)
	h.publisher.Publish(h.channel, friendEvent{
		TargetUserID: targetUserID,
		Type:         eventType,
		Payload:      json.RawMessage(data),
	})
}

func (h *Handler) publishAudit(ctx context.Context, event audit.Event) {
	if h.auditPublisher == nil {
		return
	}
	_ = h.auditPublisher.Publish(ctx, event)
}

func actorIDFromHeader(r *stdhttp.Request) (string, bool) {
	actorID := r.Header.Get("X-User-ID")
	return actorID, actorID != ""
}

func mapFriend(friend Friend) friendResponse {
	return friendResponse{
		ID:        friend.ID,
		UserID:    friend.UserID,
		FriendID:  friend.FriendID,
		CreatedAt: friend.CreatedAt.UTC().Format(timeRFC3339),
	}
}

func mapFriendRequest(request FriendRequest) friendRequestResponse {
	return friendRequestResponse{
		ID:         request.ID,
		SenderID:   request.SenderID,
		ReceiverID: request.ReceiverID,
		CreatedAt:  request.CreatedAt.UTC().Format(timeRFC3339),
	}
}

func handleError(w stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidActorID),
		errors.Is(err, ErrInvalidReceiverID),
		errors.Is(err, ErrInvalidFriendID),
		errors.Is(err, ErrInvalidRequestID),
		errors.Is(err, ErrCannotSendRequestToYourself):
		writeJSONError(w, stdhttp.StatusBadRequest, err.Error())
	case errors.Is(err, ErrAlreadyFriends), errors.Is(err, ErrFriendRequestAlreadyExists):
		writeJSONError(w, stdhttp.StatusConflict, err.Error())
	case errors.Is(err, ErrFriendRequestNotFound), errors.Is(err, ErrFriendNotFound):
		writeJSONError(w, stdhttp.StatusNotFound, err.Error())
	case errors.Is(err, ErrUnauthorizedFriendRequest):
		writeJSONError(w, stdhttp.StatusForbidden, err.Error())
	default:
		writeJSONError(w, stdhttp.StatusInternalServerError, fmt.Sprintf("internal error: %v", err))
	}
}

func writeJSONError(w stdhttp.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]string{"error": message})
}

func writeJSON(w stdhttp.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
