package auth

import (
	"context"
	"errors"
	"fmt"

	authpb "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal/pb/auth"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

var ErrUserAlredyExists = errors.New("User Alredy Exists")

func (s *Service) Register(req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	if user, _ := s.repo.FindByUsername(context.Background(), req.Username); user != nil {
		return &authpb.RegisterResponse{}, ErrUserAlredyExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return &authpb.RegisterResponse{}, fmt.Errorf("failed to hash password: %w", err)
	}
	req.Password = string(hash)
	userCreated, err := s.repo.Create(context.Background(), req)
	if err != nil {
		return &authpb.RegisterResponse{}, err
	}
	println(userCreated.ID)
	token, err := createToken(userCreated.ID)
	return &authpb.RegisterResponse{Token: token}, err
}

func (s *Service) Authenticate(req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	user, err := s.repo.FindByUsername(context.Background(), req.Username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return &authpb.LoginResponse{}, fmt.Errorf("invalid credentials")
	}
	token, err := createToken(user.ID)
	return &authpb.LoginResponse{Token: token}, err
}
