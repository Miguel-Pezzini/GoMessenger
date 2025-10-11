package auth

import (
	"encoding/json"
	"net/http"

	authpb "github.com/Miguel-Pezzini/real_time_chat/gateway/internal/pb"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
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
	w.Header().Set("Content-Type", "application/json")
	var req authpb.LoginRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	token, err := h.service.Authenticate(r.Context(), req)
	if err != nil {
		http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(AuthResponse{Token: token})
}

func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req authpb.RegisterRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	token, err := h.service.Register(r.Context(), req)
	switch {
	case err == ErrUserAlredyExists:
		http.Error(w, `{"error":"User already exists"}`, http.StatusForbidden)
	case err != nil:
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(AuthResponse{Token: token})
	}
}
