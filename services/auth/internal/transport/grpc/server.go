package grpc

import (
	"context"
	"errors"
	"fmt"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"github.com/Miguel-Pezzini/GoMessenger/services/auth/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	authpb.UnimplementedAuthServiceServer
	service *domain.Service
}

func NewServer(service *domain.Service) *Server {
	return &Server{service: service}
}

func (s *Server) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	res, err := s.service.Register(ctx, req)
	if err != nil {
		return nil, mapError(err)
	}
	return res, nil
}

func (s *Server) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	res, err := s.service.Authenticate(ctx, req)
	if err != nil {
		return nil, mapError(err)
	}
	return res, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidUsername), errors.Is(err, domain.ErrInvalidPassword):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrUserAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}
