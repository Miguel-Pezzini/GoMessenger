package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
)

type handlerRepoStub struct {
	createFn         func(context.Context, *RegisterRequest) (*User, error)
	findByUsernameFn func(context.Context, string) (*User, error)
}

func (r handlerRepoStub) Create(ctx context.Context, req *RegisterRequest) (*User, error) {
	return r.createFn(ctx, req)
}

func (r handlerRepoStub) FindByUsername(ctx context.Context, username string) (*User, error) {
	return r.findByUsernameFn(ctx, username)
}

func (handlerRepoStub) FindByFriendCode(context.Context, string) (*User, error) {
	return nil, ErrUserNotFound
}

func (handlerRepoStub) FindByID(context.Context, string) (*User, error) {
	return nil, ErrUserNotFound
}

func (handlerRepoStub) SetFriendCode(context.Context, string, string) error {
	return nil
}

type handlerAuditPublisherStub struct{}

func (handlerAuditPublisherStub) Publish(context.Context, audit.Event) error { return nil }

func TestRegisterRejectsUnknownFields(t *testing.T) {
	handler := NewHandler(NewService(handlerRepoStub{}, NewTokenIssuer("secret", testJWTExpiry)), handlerAuditPublisherStub{}, "")
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`{"username":"alice","password":"secret","extra":true}`))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "invalid payload" {
		t.Fatalf("expected invalid payload, got %q", body["error"])
	}
}

func TestLoginRejectsEmptyBody(t *testing.T) {
	handler := NewHandler(NewService(handlerRepoStub{}, NewTokenIssuer("secret", testJWTExpiry)), handlerAuditPublisherStub{}, "")
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(nil))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterReturnsGenericInternalError(t *testing.T) {
	handler := NewHandler(NewService(handlerRepoStub{
		findByUsernameFn: func(context.Context, string) (*User, error) {
			return nil, errors.New("mongo timeout")
		},
		createFn: func(context.Context, *RegisterRequest) (*User, error) {
			t.Fatal("create should not be called")
			return nil, nil
		},
	}, NewTokenIssuer("secret", testJWTExpiry)), handlerAuditPublisherStub{}, "")
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`{"username":"alice","password":"secret"}`))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Fatalf("expected generic internal error, got %q", body["error"])
	}
}

func TestBootstrapAdminRequiresInternalToken(t *testing.T) {
	handler := NewHandler(NewService(handlerRepoStub{}, NewTokenIssuer("secret", testJWTExpiry)), handlerAuditPublisherStub{}, "internal")
	req := httptest.NewRequest(http.MethodPost, "/internal/users/admin", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	rec := httptest.NewRecorder()

	handler.BootstrapAdmin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestBootstrapAdminCreatesAdmin(t *testing.T) {
	var capturedRole string
	handler := NewHandler(NewService(handlerRepoStub{
		findByUsernameFn: func(context.Context, string) (*User, error) {
			return nil, ErrUserNotFound
		},
		createFn: func(_ context.Context, req *RegisterRequest) (*User, error) {
			capturedRole = req.Role
			return &User{ID: "admin-1", Username: req.Username, Role: req.Role, FriendCode: req.FriendCode}, nil
		},
	}, NewTokenIssuer("secret", testJWTExpiry)), handlerAuditPublisherStub{}, "internal")
	req := httptest.NewRequest(http.MethodPost, "/internal/users/admin", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	req.Header.Set("X-Internal-Token", "internal")
	rec := httptest.NewRecorder()

	handler.BootstrapAdmin(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if capturedRole != RoleAdmin {
		t.Fatalf("expected role %s, got %s", RoleAdmin, capturedRole)
	}

	var body tokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Role != RoleAdmin {
		t.Fatalf("expected response role %s, got %s", RoleAdmin, body.Role)
	}
}
