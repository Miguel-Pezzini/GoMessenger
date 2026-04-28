package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AuthHTTPClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

func NewAuthHTTPClient(baseURL, internalToken string) *AuthHTTPClient {
	return &AuthHTTPClient{
		baseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		internalToken: strings.TrimSpace(internalToken),
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *AuthHTTPClient) LookupUsernameByUserID(ctx context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", fmt.Errorf("empty user id")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/internal/users/username/"+url.PathEscape(userID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("user not found")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("auth username lookup unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth username lookup failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	out.Username = strings.TrimSpace(out.Username)
	if out.Username == "" {
		return "", fmt.Errorf("empty username in auth response")
	}

	return out.Username, nil
}
