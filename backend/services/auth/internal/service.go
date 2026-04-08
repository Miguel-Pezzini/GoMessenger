package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		return nil, ErrInvalidUsername
	}
	if strings.TrimSpace(req.Password) == "" {
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

	role := req.Role
	if role != RoleAdmin {
		role = RoleUser
	}

	createReq := &RegisterRequest{
		Username: req.Username,
		Password: string(hash),
		Role:     role,
	}
	userCreated, err := s.repo.Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Create(userCreated.ID, userCreated.Role)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}

	return &RegisterResponse{Token: token, Role: userCreated.Role}, nil
}

func (s *Service) Authenticate(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		return nil, ErrInvalidUsername
	}
	if strings.TrimSpace(req.Password) == "" {
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

	token, err := s.tokens.Create(user.ID, user.Role)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}

	return &LoginResponse{Token: token, Role: user.Role}, nil
}
