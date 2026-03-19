package domain

import (
	"context"
	"time"
)

type Repository interface {
	CreateFriendRequest(ctx context.Context, request FriendRequest) (FriendRequest, error)
	GetFriendRequestByID(ctx context.Context, requestID string) (FriendRequest, error)
	ListPendingFriendRequests(ctx context.Context, receiverID string) ([]FriendRequest, error)
	DeleteFriendRequestByID(ctx context.Context, requestID string) error
	FriendRequestExistsBetween(ctx context.Context, firstUserID, secondUserID string) (bool, error)
	FriendshipExists(ctx context.Context, userID, friendID string) (bool, error)
	CreateFriendships(ctx context.Context, firstUserID, secondUserID string, createdAt time.Time) error
	DeleteFriendships(ctx context.Context, firstUserID, secondUserID string) error
	ListFriends(ctx context.Context, userID string) ([]Friend, error)
	RunInTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}
