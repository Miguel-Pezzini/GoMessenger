package friends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"strings"
	"time"
)

// AuthHTTPClient calls the auth service internal lookup API (not exposed through the gateway).
type AuthHTTPClient struct {
	baseURL       string
	internalToken string
	httpClient    *stdhttp.Client
}

func NewAuthHTTPClient(baseURL, internalToken string) *AuthHTTPClient {
	return &AuthHTTPClient{
		baseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		internalToken: strings.TrimSpace(internalToken),
		httpClient:    &stdhttp.Client{Timeout: 10 * time.Second},
	}
}

func (c *AuthHTTPClient) LookupUserIDByFriendCode(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodGet,
		c.baseURL+"/internal/users/by-friend-code/"+url.PathEscape(code), nil)
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

	if resp.StatusCode == stdhttp.StatusNotFound {
		return "", ErrUnknownFriendCode
	}
	if resp.StatusCode == stdhttp.StatusUnauthorized {
		return "", fmt.Errorf("auth lookup unauthorized")
	}
	if resp.StatusCode != stdhttp.StatusOK {
		return "", fmt.Errorf("auth lookup failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.UserID) == "" {
		return "", ErrUnknownFriendCode
	}

	return out.UserID, nil
}
