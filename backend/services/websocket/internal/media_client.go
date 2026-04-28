package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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

func (c *MediaHTTPClient) PrepareMessage(senderID, receiverID string, attachmentIDs []string) ([]AttachmentSnapshot, error) {
	if len(attachmentIDs) == 0 {
		return []AttachmentSnapshot{}, nil
	}
	if c.baseURL == "" {
		return nil, ValidationError{Message: "media service is not configured"}
	}

	body, err := json.Marshal(map[string]any{
		"sender_id":      senderID,
		"receiver_id":    receiverID,
		"attachment_ids": attachmentIDs,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/internal/attachments/prepare-message", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		Attachments []AttachmentSnapshot `json:"attachments"`
		Error       string               `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if payload.Error != "" {
			return nil, ValidationError{Message: payload.Error}
		}
		return nil, fmt.Errorf("media prepare-message failed: %s", resp.Status)
	}

	return payload.Attachments, nil
}
