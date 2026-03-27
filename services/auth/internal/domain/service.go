package domain

import (
	"context"
	"errors"
	"fmt"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo   Repository
	tokens *TokenIssuer
}

func NewService(repo Repository, tokens *TokenIssuer) *Service {
	return &Service{repo: repo, tokens: tokens}
}

var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidUsername    = errors.New("username is required")
	ErrInvalidPassword    = errors.New("password is required")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

func (s *Service) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	if req.Username == "" {
		return nil, ErrInvalidUsername
	}
	if req.Password == "" {
		return nil, ErrInvalidPassword
	}

	if user, err := s.repo.FindByUsername(ctx, req.Username); err == nil && user != nil {
		return nil, ErrUserAlreadyExists
	} else if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, fmt.Errorf("find user by username: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	createReq := &authpb.RegisterRequest{
		Username: req.Username,
		Password: string(hash),
	}
	userCreated, err := s.repo.Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Create(userCreated.ID)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}

	return &authpb.RegisterResponse{Token: token}, nil
}

func (s *Service) Authenticate(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	if req.Username == "" {
		return nil, ErrInvalidUsername
	}
	if req.Password == "" {
		return nil, ErrInvalidPassword
	}

	user, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokens.Create(user.ID)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}

	return &authpb.LoginResponse{Token: token}, nil
}
