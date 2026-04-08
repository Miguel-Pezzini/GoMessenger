package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

type friendRequestResponse struct {
	ID         string `json:"id"`
	SenderID   string `json:"senderId"`
	ReceiverID string `json:"receiverId"`
	CreatedAt  string `json:"createdAt"`
}

type friendResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	FriendID  string `json:"friendId"`
	CreatedAt string `json:"createdAt"`
}

func TestSendFriendRequestCreatesPendingRequest(t *testing.T) {
	t.Parallel()

	senderToken, receiverToken, senderID, receiverID := newFriendTestUsers(t, "send_request")

	request := sendFriendRequest(t, senderToken, receiverID)

	if strings.TrimSpace(request.ID) == "" {
		t.Fatal("expected created friend request id")
	}
	if request.SenderID != senderID {
		t.Fatalf("expected sender %s, got %s", senderID, request.SenderID)
	}
	if request.ReceiverID != receiverID {
		t.Fatalf("expected receiver %s, got %s", receiverID, request.ReceiverID)
	}

	pendingRequests := listPendingFriendRequests(t, receiverToken)
	if !hasPendingRequest(pendingRequests, request.ID, senderID, receiverID) {
		t.Fatalf("expected pending requests to include request %+v, got %+v", request, pendingRequests)
	}
}

func TestAcceptFriendRequestCreatesFriendship(t *testing.T) {
	t.Parallel()

	senderToken, receiverToken, senderID, receiverID := newFriendTestUsers(t, "accept_request")

	request := sendFriendRequest(t, senderToken, receiverID)
	acceptFriendRequest(t, receiverToken, request.ID)

	pendingRequests := listPendingFriendRequests(t, receiverToken)
	if hasPendingRequest(pendingRequests, request.ID, senderID, receiverID) {
		t.Fatalf("expected request %s to be removed from pending list, got %+v", request.ID, pendingRequests)
	}

	senderFriends := listFriends(t, senderToken)
	if !hasFriend(senderFriends, senderID, receiverID) {
		t.Fatalf("expected sender friend list to include %s, got %+v", receiverID, senderFriends)
	}

	receiverFriends := listFriends(t, receiverToken)
	if !hasFriend(receiverFriends, receiverID, senderID) {
		t.Fatalf("expected receiver friend list to include %s, got %+v", senderID, receiverFriends)
	}
}

func TestDeclineFriendRequestRemovesPendingRequest(t *testing.T) {
	t.Parallel()

	senderToken, receiverToken, senderID, receiverID := newFriendTestUsers(t, "decline_request")

	request := sendFriendRequest(t, senderToken, receiverID)
	declineFriendRequest(t, receiverToken, request.ID)

	pendingRequests := listPendingFriendRequests(t, receiverToken)
	if hasPendingRequest(pendingRequests, request.ID, senderID, receiverID) {
		t.Fatalf("expected request %s to be removed from pending list, got %+v", request.ID, pendingRequests)
	}

	senderFriends := listFriends(t, senderToken)
	if hasFriend(senderFriends, senderID, receiverID) {
		t.Fatalf("expected sender to have no friendship with %s, got %+v", receiverID, senderFriends)
	}

	receiverFriends := listFriends(t, receiverToken)
	if hasFriend(receiverFriends, receiverID, senderID) {
		t.Fatalf("expected receiver to have no friendship with %s, got %+v", senderID, receiverFriends)
	}
}

func TestRemoveFriendDeletesFriendship(t *testing.T) {
	t.Parallel()

	senderToken, receiverToken, senderID, receiverID := newFriendTestUsers(t, "remove_friend")

	request := sendFriendRequest(t, senderToken, receiverID)
	acceptFriendRequest(t, receiverToken, request.ID)
	removeFriend(t, senderToken, receiverID)

	senderFriends := listFriends(t, senderToken)
	if hasFriend(senderFriends, senderID, receiverID) {
		t.Fatalf("expected sender friendship with %s to be removed, got %+v", receiverID, senderFriends)
	}

	receiverFriends := listFriends(t, receiverToken)
	if hasFriend(receiverFriends, receiverID, senderID) {
		t.Fatalf("expected receiver friendship with %s to be removed, got %+v", senderID, receiverFriends)
	}
}

func newFriendTestUsers(t *testing.T, prefix string) (senderToken, receiverToken, senderID, receiverID string) {
	t.Helper()

	timestamp := time.Now().UnixNano()
	password := "123456"

	senderUsername := fmt.Sprintf("friends_%s_sender_%d", prefix, timestamp)
	receiverUsername := fmt.Sprintf("friends_%s_receiver_%d", prefix, timestamp)

	senderToken = registerOrLogin(t, senderUsername, password)
	receiverToken = registerOrLogin(t, receiverUsername, password)

	senderID = extractUserIDFromJWT(t, senderToken)
	receiverID = extractUserIDFromJWT(t, receiverToken)

	return senderToken, receiverToken, senderID, receiverID
}

func sendFriendRequest(t *testing.T, token, receiverID string) friendRequestResponse {
	t.Helper()

	body := map[string]string{"receiverId": receiverID}
	resp := doAuthenticatedJSONRequest(t, http.MethodPost, "/friends/requests", token, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, resp.StatusCode, readResponseBody(t, resp))
	}

	var request friendRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
		t.Fatalf("failed to decode send friend request response: %v", err)
	}

	return request
}

func acceptFriendRequest(t *testing.T, token, requestID string) {
	t.Helper()

	resp := doAuthenticatedJSONRequest(t, http.MethodPost, "/friends/requests/"+requestID+"/accept", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.StatusCode, readResponseBody(t, resp))
	}

	var result map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode accept friend request response: %v", err)
	}
	if !result["accepted"] {
		t.Fatalf("expected accepted=true, got %+v", result)
	}
}

func declineFriendRequest(t *testing.T, token, requestID string) {
	t.Helper()

	resp := doAuthenticatedJSONRequest(t, http.MethodDelete, "/friends/requests/"+requestID+"/decline", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, resp.StatusCode, readResponseBody(t, resp))
	}
}

func removeFriend(t *testing.T, token, friendID string) {
	t.Helper()

	resp := doAuthenticatedJSONRequest(t, http.MethodDelete, "/friends/"+friendID, token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, resp.StatusCode, readResponseBody(t, resp))
	}
}

func listPendingFriendRequests(t *testing.T, token string) []friendRequestResponse {
	t.Helper()

	resp := doAuthenticatedJSONRequest(t, http.MethodGet, "/friends/requests/pending", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.StatusCode, readResponseBody(t, resp))
	}

	var requests []friendRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&requests); err != nil {
		t.Fatalf("failed to decode pending friend requests response: %v", err)
	}

	return requests
}

func listFriends(t *testing.T, token string) []friendResponse {
	t.Helper()

	resp := doAuthenticatedJSONRequest(t, http.MethodGet, "/friends", token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.StatusCode, readResponseBody(t, resp))
	}

	var friends []friendResponse
	if err := json.NewDecoder(resp.Body).Decode(&friends); err != nil {
		t.Fatalf("failed to decode friends response: %v", err)
	}

	return friends
}

func doAuthenticatedJSONRequest(t *testing.T, method, path, token string, payload any) *http.Response {
	t.Helper()

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, gatewayBaseURL+path, body)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			t.Skipf("gateway unavailable for integration test: %v", err)
		}
		t.Fatalf("failed to perform request %s %s: %v", method, path, err)
	}

	return resp
}

func readResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %v", err)
	}

	return strings.TrimSpace(string(data))
}

func hasPendingRequest(requests []friendRequestResponse, requestID, senderID, receiverID string) bool {
	for _, request := range requests {
		if request.ID == requestID && request.SenderID == senderID && request.ReceiverID == receiverID {
			return true
		}
	}
	return false
}

func hasFriend(friends []friendResponse, userID, friendID string) bool {
	for _, friend := range friends {
		if friend.UserID == userID && friend.FriendID == friendID {
			return true
		}
	}
	return false
}
