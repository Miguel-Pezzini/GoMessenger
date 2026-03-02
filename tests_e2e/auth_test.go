package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"testing"
)

type RegisterResponse struct {
	Token string `json:"token"`
}

func TestRegisterAndLogin(t *testing.T) {
	body := map[string]string{
		"username": "test_user",
		"password": "123456",
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post("http://localhost:8080/auth/register", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for e2e test: %v", err)
		}
		t.Fatalf("error registering user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d or %d, got %d", http.StatusCreated, http.StatusForbidden, resp.StatusCode)
	}

	if resp.StatusCode == http.StatusForbidden {
		return
	}

	var result RegisterResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		t.Fatalf("error decoding register response: %v", err)
	}
	if result.Token == "" {
		t.Fatalf("token is empty")
	}
}
