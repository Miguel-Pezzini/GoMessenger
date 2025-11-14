package auth

import (
	"context"

	authpb "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal/pb/auth"
)

type Repository interface {
	Create(ctx context.Context, user *authpb.RegisterRequest) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
}
