package grpc

import (
	"context"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"github.com/Miguel-Pezzini/GoMessenger/services/auth/internal/domain"
)

type Server struct {
	authpb.UnimplementedAuthServiceServer
	service *domain.Service
}

func NewServer(service *domain.Service) *Server {
	return &Server{service: service}
}

func (s *Server) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	return s.service.Register(req)
}

func (s *Server) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	return s.service.Authenticate(req)
}
