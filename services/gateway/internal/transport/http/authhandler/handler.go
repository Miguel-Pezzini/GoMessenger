package authhandler

import (
	"encoding/json"
	"net/http"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/domain/auth"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/response"
)

type Handler struct {
	service *auth.Service
}

func New(service *auth.Service) *Handler {
	return &Handler{service: service}
}

type RegisterUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req authpb.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	token, err := h.service.Authenticate(r.Context(), &req)
	if err != nil {
		response.HandleGRPCError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, AuthResponse{Token: token})
}

func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req authpb.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSONError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	token, err := h.service.Register(r.Context(), &req)
	if err != nil {
		response.HandleGRPCError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, AuthResponse{Token: token})
}
