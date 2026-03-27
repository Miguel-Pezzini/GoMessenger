package grpc

import (
	"context"
	"testing"
	"time"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"github.com/Miguel-Pezzini/GoMessenger/services/auth/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubRepository struct {
	createFn         func(context.Context, *authpb.RegisterRequest) (*domain.User, error)
	findByUsernameFn func(context.Context, string) (*domain.User, error)
}

func (r stubRepository) Create(ctx context.Context, req *authpb.RegisterRequest) (*domain.User, error) {
	return r.createFn(ctx, req)
}

func (r stubRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	return r.findByUsernameFn(ctx, username)
}

func TestRegisterMapsAlreadyExistsToGRPCConflict(t *testing.T) {
	service := domain.NewService(
		stubRepository{
			findByUsernameFn: func(context.Context, string) (*domain.User, error) {
				return &domain.User{ID: "1", Username: "alice"}, nil
			},
			createFn: func(context.Context, *authpb.RegisterRequest) (*domain.User, error) {
				t.Fatal("create should not be called when user already exists")
				return nil, nil
			},
		},
		domain.NewTokenIssuer("secret", time.Hour),
	)

	server := NewServer(service)
	_, err := server.Register(context.Background(), &authpb.RegisterRequest{Username: "alice", Password: "123456"})
	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.AlreadyExists {
		t.Fatalf("expected %v, got %v", codes.AlreadyExists, st.Code())
	}
}

func TestLoginMapsInvalidCredentialsToUnauthenticated(t *testing.T) {
	service := domain.NewService(
		stubRepository{
			findByUsernameFn: func(context.Context, string) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			createFn: func(context.Context, *authpb.RegisterRequest) (*domain.User, error) {
				t.Fatal("create should not be called on login")
				return nil, nil
			},
		},
		domain.NewTokenIssuer("secret", time.Hour),
	)

	server := NewServer(service)
	_, err := server.Login(context.Background(), &authpb.LoginRequest{Username: "alice", Password: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected %v, got %v", codes.Unauthenticated, st.Code())
	}
}
