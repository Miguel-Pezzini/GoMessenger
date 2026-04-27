package auth

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, user *RegisterRequest) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByFriendCode(ctx context.Context, friendCode string) (*User, error)
	FindByID(ctx context.Context, userIDHex string) (*User, error)
	SetFriendCode(ctx context.Context, userIDHex string, friendCode string) error
}
