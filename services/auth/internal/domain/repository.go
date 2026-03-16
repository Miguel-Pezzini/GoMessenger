package domain

import (
	"context"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
)

type Repository interface {
	Create(ctx context.Context, user *authpb.RegisterRequest) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
}
