package friends

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type repositoryStub struct {
	friendshipExistsResult bool
	friendRequestExists    bool
	friendRequest          FriendRequest
	listFriendsResult      []Friend
	listRequestsResult     []FriendRequest
	createFriendshipsCalls int
	deleteRequestsCalls    int
	deleteFriendshipsErr   error
	createdRequest         FriendRequest
}

func (r *repositoryStub) CreateFriendRequest(_ context.Context, request FriendRequest) (FriendRequest, error) {
	request.ID = "request-1"
	r.createdRequest = request
	return request, nil
}

func (r *repositoryStub) GetFriendRequestByID(_ context.Context, requestID string) (FriendRequest, error) {
	if r.friendRequest.ID == "" || r.friendRequest.ID != requestID {
		return FriendRequest{}, mongo.ErrNoDocuments
	}
	return r.friendRequest, nil
}

func (r *repositoryStub) ListPendingFriendRequests(_ context.Context, _ string) ([]FriendRequest, error) {
	return r.listRequestsResult, nil
}

func (r *repositoryStub) DeleteFriendRequestByID(_ context.Context, requestID string) error {
	if r.friendRequest.ID == "" || r.friendRequest.ID != requestID {
		return mongo.ErrNoDocuments
	}
	r.deleteRequestsCalls++
	r.friendRequest = FriendRequest{}
	return nil
}

func (r *repositoryStub) FriendRequestExistsBetween(_ context.Context, _, _ string) (bool, error) {
	return r.friendRequestExists, nil
}

func (r *repositoryStub) FriendshipExists(_ context.Context, _, _ string) (bool, error) {
	return r.friendshipExistsResult, nil
}

func (r *repositoryStub) CreateFriendships(_ context.Context, firstUserID, secondUserID string, createdAt time.Time) error {
	r.createFriendshipsCalls++
	r.listFriendsResult = []Friend{
		{ID: "friend-1", UserID: firstUserID, FriendID: secondUserID, CreatedAt: createdAt},
		{ID: "friend-2", UserID: secondUserID, FriendID: firstUserID, CreatedAt: createdAt},
	}
	r.friendshipExistsResult = true
	return nil
}

func (r *repositoryStub) DeleteFriendships(_ context.Context, _, _ string) error {
	return r.deleteFriendshipsErr
}

func (r *repositoryStub) ListFriends(_ context.Context, _ string) ([]Friend, error) {
	return r.listFriendsResult, nil
}

func TestSendFriendRequest(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo)
	now := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	request, err := service.SendFriendRequest(context.Background(), "user-1", "user-2")
	if err != nil {
		t.Fatalf("SendFriendRequest returned error: %v", err)
	}

	if request.ID == "" {
		t.Fatalf("expected created request id")
	}
	if repo.createdRequest.SenderID != "user-1" || repo.createdRequest.ReceiverID != "user-2" {
		t.Fatalf("unexpected request saved: %+v", repo.createdRequest)
	}
	if !repo.createdRequest.CreatedAt.Equal(now) {
		t.Fatalf("expected request timestamp %v, got %v", now, repo.createdRequest.CreatedAt)
	}
}

func TestSendFriendRequestPreventsDuplicatePendingRequests(t *testing.T) {
	repo := &repositoryStub{friendRequestExists: true}
	service := NewService(repo)

	_, err := service.SendFriendRequest(context.Background(), "user-1", "user-2")
	if !errors.Is(err, ErrFriendRequestAlreadyExists) {
		t.Fatalf("expected ErrFriendRequestAlreadyExists, got %v", err)
	}
}

func TestAcceptFriendRequest(t *testing.T) {
	repo := &repositoryStub{
		friendRequest: FriendRequest{
			ID:         "request-1",
			SenderID:   "user-1",
			ReceiverID: "user-2",
		},
	}
	service := NewService(repo)

	if _, err := service.AcceptFriendRequest(context.Background(), "user-2", "request-1"); err != nil {
		t.Fatalf("AcceptFriendRequest returned error: %v", err)
	}

	if repo.createFriendshipsCalls != 1 {
		t.Fatalf("expected 1 friendship creation, got %d", repo.createFriendshipsCalls)
	}
	if repo.deleteRequestsCalls != 1 {
		t.Fatalf("expected 1 request deletion, got %d", repo.deleteRequestsCalls)
	}
}

func TestAcceptFriendRequestSkipsFriendCreationWhenUsersAlreadyFriends(t *testing.T) {
	repo := &repositoryStub{
		friendshipExistsResult: true,
		friendRequest: FriendRequest{
			ID:         "request-1",
			SenderID:   "user-1",
			ReceiverID: "user-2",
		},
	}
	service := NewService(repo)

	if _, err := service.AcceptFriendRequest(context.Background(), "user-2", "request-1"); err != nil {
		t.Fatalf("AcceptFriendRequest returned error: %v", err)
	}

	if repo.createFriendshipsCalls != 0 {
		t.Fatalf("expected no friendship creation, got %d", repo.createFriendshipsCalls)
	}
	if repo.deleteRequestsCalls != 1 {
		t.Fatalf("expected 1 request deletion, got %d", repo.deleteRequestsCalls)
	}
}

func TestDeclineFriendRequest(t *testing.T) {
	repo := &repositoryStub{
		friendRequest: FriendRequest{
			ID:         "request-1",
			SenderID:   "user-1",
			ReceiverID: "user-2",
		},
	}
	service := NewService(repo)

	if _, err := service.DeclineFriendRequest(context.Background(), "user-2", "request-1"); err != nil {
		t.Fatalf("DeclineFriendRequest returned error: %v", err)
	}

	if repo.deleteRequestsCalls != 1 {
		t.Fatalf("expected 1 request deletion, got %d", repo.deleteRequestsCalls)
	}
}

func TestAcceptFriendRequestRejectsUnauthorizedActor(t *testing.T) {
	repo := &repositoryStub{
		friendRequest: FriendRequest{
			ID:         "request-1",
			SenderID:   "user-1",
			ReceiverID: "user-2",
		},
	}
	service := NewService(repo)

	_, err := service.AcceptFriendRequest(context.Background(), "user-3", "request-1")
	if !errors.Is(err, ErrUnauthorizedFriendRequest) {
		t.Fatalf("expected ErrUnauthorizedFriendRequest, got %v", err)
	}
}

func TestDeclineFriendRequestRejectsUnauthorizedActor(t *testing.T) {
	repo := &repositoryStub{
		friendRequest: FriendRequest{
			ID:         "request-1",
			SenderID:   "user-1",
			ReceiverID: "user-2",
		},
	}
	service := NewService(repo)

	_, err := service.DeclineFriendRequest(context.Background(), "user-3", "request-1")
	if !errors.Is(err, ErrUnauthorizedFriendRequest) {
		t.Fatalf("expected ErrUnauthorizedFriendRequest, got %v", err)
	}
}

func TestSendFriendRequestPreventsSelfRequest(t *testing.T) {
	service := NewService(&repositoryStub{})

	_, err := service.SendFriendRequest(context.Background(), "user-1", "user-1")
	if !errors.Is(err, ErrCannotSendRequestToYourself) {
		t.Fatalf("expected ErrCannotSendRequestToYourself, got %v", err)
	}
}
