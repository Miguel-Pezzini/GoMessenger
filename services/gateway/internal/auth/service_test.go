package auth

import (
	"context"
	"errors"
	"testing"

	authpb "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/pb/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeAuthClient struct {
	registerFn func(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.RegisterResponse, error)
	loginFn    func(context.Context, *authpb.LoginRequest, ...grpc.CallOption) (*authpb.LoginResponse, error)
}

func (f fakeAuthClient) Register(ctx context.Context, req *authpb.RegisterRequest, opts ...grpc.CallOption) (*authpb.RegisterResponse, error) {
	return f.registerFn(ctx, req, opts...)
}

func (f fakeAuthClient) Login(ctx context.Context, req *authpb.LoginRequest, opts ...grpc.CallOption) (*authpb.LoginResponse, error) {
	return f.loginFn(ctx, req, opts...)
}

func TestRegisterMapsAlreadyExistsError(t *testing.T) {
	svc := NewService(fakeAuthClient{
		registerFn: func(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.RegisterResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "User Alredy Exists")
		},
		loginFn: func(context.Context, *authpb.LoginRequest, ...grpc.CallOption) (*authpb.LoginResponse, error) {
			return &authpb.LoginResponse{}, nil
		},
	})

	_, err := svc.Register(context.Background(), &authpb.RegisterRequest{})
	if !errors.Is(err, ErrUserAlredyExists) {
		t.Fatalf("expected ErrUserAlredyExists, got: %v", err)
	}
}

func TestRegisterReturnsErrorOnNilResponse(t *testing.T) {
	svc := NewService(fakeAuthClient{
		registerFn: func(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.RegisterResponse, error) {
			return nil, nil
		},
		loginFn: func(context.Context, *authpb.LoginRequest, ...grpc.CallOption) (*authpb.LoginResponse, error) {
			return &authpb.LoginResponse{}, nil
		},
	})

	_, err := svc.Register(context.Background(), &authpb.RegisterRequest{})
	if err == nil {
		t.Fatalf("expected an error for nil response")
	}
}
