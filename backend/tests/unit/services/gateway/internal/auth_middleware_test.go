package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		query    string
		expected string
	}{
		{
			name:     "from Authorization header",
			header:   "Bearer my-token",
			expected: "my-token",
		},
		{
			name:     "from query parameter",
			query:    "my-token",
			expected: "my-token",
		},
		{
			name:     "header takes precedence over query",
			header:   "Bearer header-token",
			query:    "query-token",
			expected: "header-token",
		},
		{
			name:     "missing both returns empty",
			expected: "",
		},
		{
			name:     "Authorization header without Bearer prefix returns empty and falls through to query",
			header:   "Basic abc",
			query:    "fallback-token",
			expected: "fallback-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://example.com/ws"
			if tt.query != "" {
				url += "?token=" + tt.query
			}
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := extractToken(req)
			if got != tt.expected {
				t.Errorf("extractToken() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestWrapSetsUserIDAndRoleInContext(t *testing.T) {
	middleware := New("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/presence/friend-1", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(t, "user-123", "USER", "test-secret", time.Hour))
	rec := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, _ := r.Context().Value(UserIDKey).(string); got != "user-123" {
			t.Fatalf("expected user id user-123, got %q", got)
		}
		if got, _ := r.Context().Value(UserRoleKey).(string); got != "USER" {
			t.Fatalf("expected role USER, got %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	middleware.Wrap(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestWrapAdminRejectsNonAdminRole(t *testing.T) {
	middleware := New("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(t, "user-123", "USER", "test-secret", time.Hour))
	rec := httptest.NewRecorder()

	middleware.WrapAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "forbidden" {
		t.Fatalf("expected forbidden error, got %q", body["error"])
	}
}

func TestWrapAdminAllowsAdminRole(t *testing.T) {
	middleware := New("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	req.Header.Set("Authorization", "Bearer "+makeToken(t, "admin-1", "ADMIN", "test-secret", time.Hour))
	rec := httptest.NewRecorder()

	middleware.WrapAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, _ := r.Context().Value(UserRoleKey).(string); got != "ADMIN" {
			t.Fatalf("expected role ADMIN, got %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestWrapRejectsMissingTokenWithJSONError(t *testing.T) {
	middleware := New("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/presence/friend-1", nil)
	rec := httptest.NewRecorder()

	middleware.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "missing token" {
		t.Fatalf("expected missing token error, got %q", body["error"])
	}
}
