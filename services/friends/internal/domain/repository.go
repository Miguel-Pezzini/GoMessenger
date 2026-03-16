package domain

import "context"

type Repository interface {
	Create(ctx context.Context, friend Friend) (Friend, error)
	GetByID(ctx context.Context, ownerID, id string) (Friend, error)
	ListByOwner(ctx context.Context, ownerID string) ([]Friend, error)
	Update(ctx context.Context, ownerID, id string, friend Friend) (Friend, error)
	Delete(ctx context.Context, ownerID, id string) error
}
