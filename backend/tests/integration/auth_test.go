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

type errorResponse struct {
	Error string `json:"error"`
}

func TestRegisterAndLogin(t *testing.T) {
	body := map[string]string{
		"username": "test_user",
		"password": "123456",
	}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(gatewayBaseURL+"/auth/register", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for integration test: %v", err)
		}
		t.Fatalf("error registering user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status code %d or %d, got %d", http.StatusCreated, http.StatusConflict, resp.StatusCode)
	}

	if resp.StatusCode == http.StatusConflict {
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

func TestRegisterRejectsUnknownFields(t *testing.T) {
	body := []byte(`{"username":"invalid_payload_user","password":"123456","extra":true}`)

	resp, err := http.Post(gatewayBaseURL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for integration test: %v", err)
		}
		t.Fatalf("error registering user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var result errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if result.Error != "invalid payload" {
		t.Fatalf("expected invalid payload, got %q", result.Error)
	}
}
