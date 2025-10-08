package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var UserAlredyExistsErr = errors.New("User Alredy Exists")

func (s *Service) Register(req RegisterUserRequest) (string, error) {
	if user, _ := s.repo.FindByUsername(context.Background(), req.Username); user != nil {
		return "", UserAlredyExistsErr
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	if err := s.repo.Create(context.Background(), &User{Username: req.Username, Password: string(hash)}); err != nil {
		return "", err
	}
	return createToken(req.Username)
}

func (s *Service) Authenticate(req LoginUserRequest) (string, error) {
	user, err := s.repo.FindByUsername(context.Background(), req.Username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	return createToken(req.Username)
}
