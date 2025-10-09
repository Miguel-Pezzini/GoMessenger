package auth

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, user *RegisterUserRequest) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
}
