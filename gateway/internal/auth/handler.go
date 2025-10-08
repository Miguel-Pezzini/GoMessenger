package auth

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req LoginUserRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	token, err := h.service.Authenticate(req)
	if err != nil {
		http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(AuthResponse{Token: token})
}

func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req RegisterUserRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	token, err := h.service.Register(req)
	switch {
	case err == UserAlredyExistsErr:
		http.Error(w, `{"error":"User already exists"}`, http.StatusForbidden)
	case err != nil:
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(AuthResponse{Token: token})
	}
}
