package authhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/domain/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubAuthClient struct {
	registerFn func(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.RegisterResponse, error)
	loginFn    func(context.Context, *authpb.LoginRequest, ...grpc.CallOption) (*authpb.LoginResponse, error)
}

func (c stubAuthClient) Register(ctx context.Context, req *authpb.RegisterRequest, opts ...grpc.CallOption) (*authpb.RegisterResponse, error) {
	return c.registerFn(ctx, req, opts...)
}

func (c stubAuthClient) Login(ctx context.Context, req *authpb.LoginRequest, opts ...grpc.CallOption) (*authpb.LoginResponse, error) {
	return c.loginFn(ctx, req, opts...)
}

func TestRegisterHandlerReturnsConflictForExistingUser(t *testing.T) {
	handler := New(auth.NewService(
		stubAuthClient{
			registerFn: func(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.RegisterResponse, error) {
				return nil, status.Error(codes.AlreadyExists, "user already exists")
			},
			loginFn: func(context.Context, *authpb.LoginRequest, ...grpc.CallOption) (*authpb.LoginResponse, error) {
				return nil, nil
			},
		},
	))

	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"username":"alice","password":"123456"}`))
	rec := httptest.NewRecorder()

	handler.RegisterHandler(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "user already exists" {
		t.Fatalf("unexpected error message: %q", body["error"])
	}
}
