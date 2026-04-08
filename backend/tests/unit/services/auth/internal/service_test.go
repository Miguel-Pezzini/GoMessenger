package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- stubs ---

type repoStub struct {
	createFn         func(context.Context, *RegisterRequest) (*User, error)
	findByUsernameFn func(context.Context, string) (*User, error)
}

func (r repoStub) Create(ctx context.Context, req *RegisterRequest) (*User, error) {
	return r.createFn(ctx, req)
}

func (r repoStub) FindByUsername(ctx context.Context, username string) (*User, error) {
	return r.findByUsernameFn(ctx, username)
}

func notCalled(t *testing.T, name string) func(context.Context, *RegisterRequest) (*User, error) {
	t.Helper()
	return func(context.Context, *RegisterRequest) (*User, error) {
		t.Fatalf("%s should not be called", name)
		return nil, nil
	}
}

// --- Register ---

func TestRegisterEmptyUsernameReturnsError(t *testing.T) {
	svc := NewService(repoStub{}, NewTokenIssuer("secret", time.Hour))
	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "", Password: "pass"})
	if !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("expected ErrInvalidUsername, got %v", err)
	}
}

func TestRegisterEmptyPasswordReturnsError(t *testing.T) {
	svc := NewService(repoStub{}, NewTokenIssuer("secret", time.Hour))
	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: ""})
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestRegisterDuplicateUsernameReturnsAlreadyExists(t *testing.T) {
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return &User{ID: "1", Username: "alice"}, nil
		},
		createFn: notCalled(t, "Create"),
	}, NewTokenIssuer("secret", time.Hour))

	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: "pass"})
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestRegisterSuccessReturnsToken(t *testing.T) {
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, ErrUserNotFound
		},
		createFn: func(_ context.Context, req *RegisterRequest) (*User, error) {
			return &User{ID: "user-1", Username: "alice", Role: req.Role}, nil
		},
	}, NewTokenIssuer("secret", time.Hour))

	resp, err := svc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: "pass"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected a non-empty token")
	}
	if resp.Role != RoleUser {
		t.Fatalf("expected role=%s, got %s", RoleUser, resp.Role)
	}
}

func TestRegisterFindByUsernameUnexpectedErrorPropagates(t *testing.T) {
	dbErr := errors.New("db connection lost")
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, dbErr
		},
		createFn: notCalled(t, "Create"),
	}, NewTokenIssuer("secret", time.Hour))

	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: "pass"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if errors.Is(err, ErrUserAlreadyExists) {
		t.Fatal("expected wrapped db error, not ErrUserAlreadyExists")
	}
}

func TestRegisterCreateUnexpectedErrorPropagates(t *testing.T) {
	dbErr := errors.New("insert failed")
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, ErrUserNotFound
		},
		createFn: func(_ context.Context, _ *RegisterRequest) (*User, error) {
			return nil, dbErr
		},
	}, NewTokenIssuer("secret", time.Hour))

	_, err := svc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: "pass"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped create error, got %v", err)
	}
}

// --- Authenticate ---

func TestAuthenticateEmptyUsernameReturnsError(t *testing.T) {
	svc := NewService(repoStub{}, NewTokenIssuer("secret", time.Hour))
	_, err := svc.Authenticate(context.Background(), &LoginRequest{Username: "", Password: "pass"})
	if !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("expected ErrInvalidUsername, got %v", err)
	}
}

func TestAuthenticateEmptyPasswordReturnsError(t *testing.T) {
	svc := NewService(repoStub{}, NewTokenIssuer("secret", time.Hour))
	_, err := svc.Authenticate(context.Background(), &LoginRequest{Username: "alice", Password: ""})
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestAuthenticateUserNotFoundReturnsInvalidCredentials(t *testing.T) {
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, ErrUserNotFound
		},
	}, NewTokenIssuer("secret", time.Hour))

	_, err := svc.Authenticate(context.Background(), &LoginRequest{Username: "alice", Password: "pass"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateWrongPasswordReturnsInvalidCredentials(t *testing.T) {
	// Register first to get a properly hashed password stored.
	var storedHash string
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, ErrUserNotFound
		},
		createFn: func(_ context.Context, req *RegisterRequest) (*User, error) {
			storedHash = req.Password
			return &User{ID: "1", Username: "alice", Password: storedHash, Role: req.Role}, nil
		},
	}, NewTokenIssuer("secret", time.Hour))
	_, _ = svc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: "correct"})

	// Now authenticate with wrong password.
	svc2 := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return &User{ID: "1", Username: "alice", Password: storedHash}, nil
		},
	}, NewTokenIssuer("secret", time.Hour))

	_, err := svc2.Authenticate(context.Background(), &LoginRequest{Username: "alice", Password: "wrong"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateCorrectCredentialsReturnsToken(t *testing.T) {
	var storedHash string
	registerSvc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, ErrUserNotFound
		},
		createFn: func(_ context.Context, req *RegisterRequest) (*User, error) {
			storedHash = req.Password
			return &User{ID: "1", Username: "alice", Password: storedHash, Role: req.Role}, nil
		},
	}, NewTokenIssuer("secret", time.Hour))
	_, _ = registerSvc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: "correct"})

	loginSvc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return &User{ID: "1", Username: "alice", Password: storedHash, Role: RoleUser}, nil
		},
	}, NewTokenIssuer("secret", time.Hour))

	resp, err := loginSvc.Authenticate(context.Background(), &LoginRequest{Username: "alice", Password: "correct"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected a non-empty token")
	}
	if resp.Role != RoleUser {
		t.Fatalf("expected role=%s, got %s", RoleUser, resp.Role)
	}
}

func TestRegisterDefaultsToUserRole(t *testing.T) {
	var capturedRole string
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, ErrUserNotFound
		},
		createFn: func(_ context.Context, req *RegisterRequest) (*User, error) {
			capturedRole = req.Role
			return &User{ID: "1", Username: "alice", Role: req.Role}, nil
		},
	}, NewTokenIssuer("secret", time.Hour))

	resp, err := svc.Register(context.Background(), &RegisterRequest{Username: "alice", Password: "pass"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedRole != RoleUser {
		t.Fatalf("expected repo to receive role=%s, got %s", RoleUser, capturedRole)
	}
	if resp.Role != RoleUser {
		t.Fatalf("expected response role=%s, got %s", RoleUser, resp.Role)
	}
}

func TestRegisterAdminRoleIsPreserved(t *testing.T) {
	var capturedRole string
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, ErrUserNotFound
		},
		createFn: func(_ context.Context, req *RegisterRequest) (*User, error) {
			capturedRole = req.Role
			return &User{ID: "1", Username: "admin", Role: req.Role}, nil
		},
	}, NewTokenIssuer("secret", time.Hour))

	resp, err := svc.Register(context.Background(), &RegisterRequest{Username: "admin", Password: "pass", Role: RoleAdmin})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedRole != RoleAdmin {
		t.Fatalf("expected repo to receive role=%s, got %s", RoleAdmin, capturedRole)
	}
	if resp.Role != RoleAdmin {
		t.Fatalf("expected response role=%s, got %s", RoleAdmin, resp.Role)
	}
}

func TestAuthenticateRepositoryErrorPropagates(t *testing.T) {
	dbErr := errors.New("mongo timeout")
	svc := NewService(repoStub{
		findByUsernameFn: func(_ context.Context, _ string) (*User, error) {
			return nil, dbErr
		},
	}, NewTokenIssuer("secret", time.Hour))

	_, err := svc.Authenticate(context.Background(), &LoginRequest{Username: "alice", Password: "pass"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped repository error, got %v", err)
	}
}
