package auth

import (
	"context"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"google.golang.org/grpc"
)

type ServiceClient interface {
	Register(ctx context.Context, in *authpb.RegisterRequest, opts ...grpc.CallOption) (*authpb.RegisterResponse, error)
	Login(ctx context.Context, in *authpb.LoginRequest, opts ...grpc.CallOption) (*authpb.LoginResponse, error)
}

type Service struct {
	client ServiceClient
}

func NewService(client ServiceClient) *Service {
	return &Service{client: client}
}

func (s *Service) Register(ctx context.Context, req *authpb.RegisterRequest) (string, error) {
	res, err := s.client.Register(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Token, nil
}

func (s *Service) Authenticate(ctx context.Context, req *authpb.LoginRequest) (string, error) {
	res, err := s.client.Login(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Token, err
}
