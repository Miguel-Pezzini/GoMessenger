package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrInvalidOwnerID  = errors.New("owner_id is required")
	ErrInvalidUsername = errors.New("username is required")
	ErrFriendNotFound  = errors.New("friend not found")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, ownerID, username, name string) (Friend, error) {
	ownerID = strings.TrimSpace(ownerID)
	username = strings.TrimSpace(username)
	name = strings.TrimSpace(name)
	if ownerID == "" {
		return Friend{}, ErrInvalidOwnerID
	}
	if username == "" {
		return Friend{}, ErrInvalidUsername
	}

	now := time.Now().UTC()
	return s.repo.Create(ctx, Friend{
		OwnerID:   ownerID,
		Username:  username,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *Service) GetByID(ctx context.Context, ownerID, id string) (Friend, error) {
	friend, err := s.repo.GetByID(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(id))
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Friend{}, ErrFriendNotFound
	}
	return friend, err
}

func (s *Service) ListByOwner(ctx context.Context, ownerID string) ([]Friend, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, ErrInvalidOwnerID
	}
	return s.repo.ListByOwner(ctx, ownerID)
}

func (s *Service) Update(ctx context.Context, ownerID, id, username, name string) (Friend, error) {
	current, err := s.GetByID(ctx, ownerID, id)
	if err != nil {
		return Friend{}, err
	}

	username = strings.TrimSpace(username)
	name = strings.TrimSpace(name)
	if username != "" {
		current.Username = username
	}
	if name != "" {
		current.Name = name
	}
	if current.Username == "" {
		return Friend{}, ErrInvalidUsername
	}

	current.UpdatedAt = time.Now().UTC()
	updated, err := s.repo.Update(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(id), current)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Friend{}, ErrFriendNotFound
	}
	return updated, err
}

func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	err := s.repo.Delete(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(id))
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrFriendNotFound
	}
	return err
}
