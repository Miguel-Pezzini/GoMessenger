package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type MediaBinder interface {
	BindMessage(ctx context.Context, messageID, senderID, receiverID string, attachments []AttachmentSnapshot) error
}

type MediaHTTPClient struct {
	baseURL       string
	internalToken string
	client        *http.Client
}

func NewMediaHTTPClient(baseURL, internalToken string) *MediaHTTPClient {
	return &MediaHTTPClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		client:        &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *MediaHTTPClient) BindMessage(ctx context.Context, messageID, senderID, receiverID string, attachments []AttachmentSnapshot) error {
	if len(attachments) == 0 {
		return nil
	}
	if c.baseURL == "" {
		return fmt.Errorf("media service is not configured")
	}

	ids := make([]string, len(attachments))
	for i, attachment := range attachments {
		ids[i] = attachment.ID
	}

	body, err := json.Marshal(map[string]any{
		"message_id":     messageID,
		"sender_id":      senderID,
		"receiver_id":    receiverID,
		"attachment_ids": ids,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/attachments/bind-message", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var payload struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Error != "" {
		return fmt.Errorf("media bind-message failed: %s", payload.Error)
	}
	return fmt.Errorf("media bind-message failed: %s", resp.Status)
}
