package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"

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
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidUsername     = errors.New("username is required")
	ErrInvalidPassword     = errors.New("password is required")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrFriendCodeRequired  = errors.New("friend_code is required")
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

	var userCreated *User
	for attempt := 0; attempt < 25; attempt++ {
		code, genErr := generateFriendCode()
		if genErr != nil {
			return nil, fmt.Errorf("friend code: %w", genErr)
		}

		createReq := &RegisterRequest{
			Username:   req.Username,
			Password:   string(hash),
			Role:       role,
			FriendCode: code,
		}
		userCreated, err = s.repo.Create(ctx, createReq)
		if err == nil {
			break
		}
		if mongo.IsDuplicateKeyError(err) {
			continue
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	if userCreated == nil {
		return nil, fmt.Errorf("create user: could not allocate unique friend code")
	}

	token, err := s.tokens.Create(userCreated.ID, userCreated.Role)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}

	return &RegisterResponse{Token: token, Role: userCreated.Role, FriendCode: userCreated.FriendCode}, nil
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

	if user.FriendCode == "" {
		for attempt := 0; attempt < 25; attempt++ {
			code, genErr := generateFriendCode()
			if genErr != nil {
				return nil, fmt.Errorf("friend code: %w", genErr)
			}
			err := s.repo.SetFriendCode(ctx, user.ID, code)
			if err == nil {
				user.FriendCode = code
				break
			}
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			return nil, fmt.Errorf("assign friend code: %w", err)
		}
		if user.FriendCode == "" {
			return nil, fmt.Errorf("assign friend code: could not allocate unique code")
		}
	}

	token, err := s.tokens.Create(user.ID, user.Role)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}

	return &LoginResponse{Token: token, Role: user.Role, FriendCode: user.FriendCode}, nil
}

func (s *Service) LookupUserIDByFriendCode(ctx context.Context, friendCode string) (string, error) {
	friendCode = strings.TrimSpace(friendCode)
	if friendCode == "" {
		return "", ErrFriendCodeRequired
	}

	user, err := s.repo.FindByFriendCode(ctx, friendCode)
	if err != nil {
		return "", err
	}

	return user.ID, nil
}

func (s *Service) GetUsernameByUserID(ctx context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", ErrUserNotFound
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return "", err
	}

	return user.Username, nil
}
