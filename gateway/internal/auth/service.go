package auth

import (
	"context"
	"errors"

	authpb "github.com/Miguel-Pezzini/real_time_chat/gateway/internal/pb/proto/auth"
)

type Service struct {
	client authpb.AuthServiceClient
}

func NewService(client authpb.AuthServiceClient) *Service {
	return &Service{client: client}
}

var ErrUserAlredyExists = errors.New("User Alredy Exists")

func (s *Service) Register(ctx context.Context, req authpb.RegisterRequest) (string, error) {
	res, err := s.client.Register(ctx, &req)
	return res.Token, err
}

func (s *Service) Authenticate(ctx context.Context, req authpb.LoginRequest) (string, error) {
	res, err := s.client.Login(ctx, &req)
	return res.Token, err
}
