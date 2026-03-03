package auth

import (
	"context"
	"errors"
	"strings"

	authpb "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/pb/auth"
	"google.golang.org/grpc/status"
)

type Service struct {
	client authpb.AuthServiceClient
}

func NewService(client authpb.AuthServiceClient) *Service {
	return &Service{client: client}
}

var ErrUserAlredyExists = errors.New("User Alredy Exists")

func (s *Service) Register(ctx context.Context, req *authpb.RegisterRequest) (string, error) {
	res, err := s.client.Register(ctx, req)
	if err != nil {
		if isUserAlreadyExistsError(err) {
			return "", ErrUserAlredyExists
		}
		return "", err
	}
	if res == nil {
		return "", errors.New("empty register response")
	}
	return res.Token, err
}

func (s *Service) Authenticate(ctx context.Context, req *authpb.LoginRequest) (string, error) {
	res, err := s.client.Login(ctx, req)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", errors.New("empty login response")
	}
	return res.Token, err
}

func isUserAlreadyExistsError(err error) bool {
	if errors.Is(err, ErrUserAlredyExists) {
		return true
	}

	statusErr, ok := status.FromError(err)
	if ok {
		return strings.Contains(strings.ToLower(statusErr.Message()), strings.ToLower(ErrUserAlredyExists.Error()))
	}

	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(ErrUserAlredyExists.Error()))
}
