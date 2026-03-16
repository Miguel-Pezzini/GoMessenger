package friends

import (
	"encoding/json"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok || ownerID == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateFriendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	friend, err := h.service.Create(r.Context(), ownerID, req)
	if err != nil {
		handleGrpcError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, friend)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok || ownerID == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	friends, err := h.service.List(r.Context(), ownerID)
	if err != nil {
		handleGrpcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, friends)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok || ownerID == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	friend, err := h.service.GetByID(r.Context(), ownerID, r.PathValue("id"))
	if err != nil {
		handleGrpcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, friend)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok || ownerID == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateFriendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	friend, err := h.service.Update(r.Context(), ownerID, r.PathValue("id"), req)
	if err != nil {
		handleGrpcError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, friend)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok || ownerID == "" {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.Delete(r.Context(), ownerID, r.PathValue("id")); err != nil {
		handleGrpcError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
