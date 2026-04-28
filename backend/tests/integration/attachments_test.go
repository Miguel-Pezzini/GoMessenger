package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"
	"time"
)

type uploadAttachmentsResponse struct {
	Attachments []attachmentResponse `json:"attachments"`
}

func TestAttachmentsFlowRealtimeHistoryAndDownloadAuthorization(t *testing.T) {
	t.Parallel()

	ts := time.Now().UnixNano()
	tokenA := registerOrLogin(t, fmt.Sprintf("attach_a_%d", ts), "123456")
	tokenB := registerOrLogin(t, fmt.Sprintf("attach_b_%d", ts), "123456")
	tokenC := registerOrLogin(t, fmt.Sprintf("attach_c_%d", ts), "123456")

	idA := extractUserIDFromJWT(t, tokenA)
	idB := extractUserIDFromJWT(t, tokenB)

	image := uploadAttachment(t, tokenA, "photo.jpg", "image/jpeg", []byte("fake-image-bytes"))
	if image.Kind != "image" {
		t.Fatalf("expected image kind, got %s", image.Kind)
	}

	connA := connectWS(t, tokenA)
	defer connA.Close()
	connB := connectWS(t, tokenB)
	defer connB.Close()

	if err := connA.WriteJSON(gatewayMessage{
		Type: "chat_message",
		Payload: chatPayload{
			ReceiverID:    idB,
			AttachmentIDs: []string{image.ID},
		},
	}); err != nil {
		t.Fatalf("failed to send attachment-only message: %v", err)
	}

	senderEcho := readMessageWithRetry(t, connA)
	receiverMessage := readMessageWithRetry(t, connB)
	assertAttachmentMessage(t, senderEcho, idA, idB, "", image.ID)
	assertAttachmentMessage(t, receiverMessage, idA, idB, "", image.ID)

	body, status := downloadAttachment(t, tokenB, image.ID)
	if status != http.StatusOK {
		t.Fatalf("expected receiver download status 200, got %d", status)
	}
	if string(body) != "fake-image-bytes" {
		t.Fatalf("unexpected downloaded body: %q", string(body))
	}

	_, status = downloadAttachment(t, tokenC, image.ID)
	if status != http.StatusForbidden {
		t.Fatalf("expected unrelated user download status 403, got %d", status)
	}

	document := uploadAttachment(t, tokenA, "notes.pdf", "application/pdf", []byte("%PDF fake"))
	if err := connA.WriteJSON(gatewayMessage{
		Type: "chat_message",
		Payload: chatPayload{
			ReceiverID:    idB,
			Content:       "see attached",
			AttachmentIDs: []string{document.ID},
		},
	}); err != nil {
		t.Fatalf("failed to send text attachment message: %v", err)
	}
	readMessageWithRetry(t, connA)
	readMessageWithRetry(t, connB)

	var history conversationResponse
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result, err := getConversation(t, tokenA, idB, "", 0)
		if err != nil {
			t.Fatalf("failed to load conversation: %v", err)
		}
		if len(result.Messages) >= 2 {
			history = result
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(history.Messages) < 2 {
		t.Fatalf("expected at least 2 messages in history, got %d", len(history.Messages))
	}

	foundImage := false
	foundDocument := false
	for _, message := range history.Messages {
		if len(message.Attachments) == 0 {
			continue
		}
		switch message.Attachments[0].ID {
		case image.ID:
			foundImage = true
			if message.Content != "" {
				t.Fatalf("expected attachment-only history message to have empty content, got %q", message.Content)
			}
		case document.ID:
			foundDocument = true
			if message.Content != "see attached" {
				t.Fatalf("expected text attachment history content, got %q", message.Content)
			}
		}
	}
	if !foundImage || !foundDocument {
		t.Fatalf("expected both attachment messages in history, found image=%v document=%v", foundImage, foundDocument)
	}
}

func uploadAttachment(t *testing.T, token, filename, contentType string, payload []byte) attachmentResponse {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("failed to write multipart part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, gatewayBaseURL+"/attachments", &body)
	if err != nil {
		t.Fatalf("failed to create upload request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to upload attachment: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected upload status 201, got %d: %s", resp.StatusCode, string(body))
	}

	var result uploadAttachmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}
	if len(result.Attachments) != 1 {
		t.Fatalf("expected one uploaded attachment, got %d", len(result.Attachments))
	}
	return result.Attachments[0]
}

func downloadAttachment(t *testing.T, token, attachmentID string) ([]byte, int) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, gatewayBaseURL+"/attachments/"+attachmentID, nil)
	if err != nil {
		t.Fatalf("failed to create download request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to download attachment: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read download body: %v", err)
	}
	return body, resp.StatusCode
}

func assertAttachmentMessage(t *testing.T, got wsMessageResponse, senderID, receiverID, content, attachmentID string) {
	t.Helper()

	if got.SenderID != senderID {
		t.Fatalf("expected sender %s, got %s", senderID, got.SenderID)
	}
	if got.ReceiverID != receiverID {
		t.Fatalf("expected receiver %s, got %s", receiverID, got.ReceiverID)
	}
	if got.Content != content {
		t.Fatalf("expected content %q, got %q", content, got.Content)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %+v", got.Attachments)
	}
	if got.Attachments[0].ID != attachmentID {
		t.Fatalf("expected attachment %s, got %+v", attachmentID, got.Attachments[0])
	}
	if got.Attachments[0].DownloadURL != "/attachments/"+attachmentID {
		t.Fatalf("unexpected download url: %s", got.Attachments[0].DownloadURL)
	}
}
