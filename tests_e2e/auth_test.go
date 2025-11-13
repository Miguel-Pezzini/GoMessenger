package main

import (
	"bytes"
	"encoding/json"
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
		t.Errorf("Error to register user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var result RegisterResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		t.Errorf("Error to decode: %v", err)
	}
	if result.Token == "" {
		t.Errorf("Token is empty")
	}
}
