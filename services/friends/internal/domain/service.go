package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrInvalidActorID              = errors.New("actor_id is required")
	ErrInvalidReceiverID           = errors.New("receiver_id is required")
	ErrInvalidFriendID             = errors.New("friend_id is required")
	ErrInvalidRequestID            = errors.New("request_id is required")
	ErrCannotSendRequestToYourself = errors.New("cannot send a friend request to yourself")
	ErrAlreadyFriends              = errors.New("users are already friends")
	ErrFriendRequestAlreadyExists  = errors.New("friend request already exists")
	ErrFriendRequestNotFound       = errors.New("friend request not found")
	ErrUnauthorizedFriendRequest   = errors.New("only the receiver can manage this friend request")
	ErrFriendNotFound              = errors.New("friend not found")
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) SendFriendRequest(ctx context.Context, senderID, receiverID string) (FriendRequest, error) {
	senderID = strings.TrimSpace(senderID)
	receiverID = strings.TrimSpace(receiverID)

	if senderID == "" {
		return FriendRequest{}, ErrInvalidActorID
	}
	if receiverID == "" {
		return FriendRequest{}, ErrInvalidReceiverID
	}
	if senderID == receiverID {
		return FriendRequest{}, ErrCannotSendRequestToYourself
	}

	alreadyFriends, err := s.repo.FriendshipExists(ctx, senderID, receiverID)
	if err != nil {
		return FriendRequest{}, err
	}
	if alreadyFriends {
		return FriendRequest{}, ErrAlreadyFriends
	}

	requestExists, err := s.repo.FriendRequestExistsBetween(ctx, senderID, receiverID)
	if err != nil {
		return FriendRequest{}, err
	}
	if requestExists {
		return FriendRequest{}, ErrFriendRequestAlreadyExists
	}

	return s.repo.CreateFriendRequest(ctx, FriendRequest{
		SenderID:   senderID,
		ReceiverID: receiverID,
		CreatedAt:  s.now(),
	})
}

func (s *Service) AcceptFriendRequest(ctx context.Context, actorID, requestID string) error {
	actorID = strings.TrimSpace(actorID)
	requestID = strings.TrimSpace(requestID)

	if actorID == "" {
		return ErrInvalidActorID
	}
	if requestID == "" {
		return ErrInvalidRequestID
	}

	request, err := s.repo.GetFriendRequestByID(ctx, requestID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrFriendRequestNotFound
	}
	if err != nil {
		return err
	}
	if request.ReceiverID != actorID {
		return ErrUnauthorizedFriendRequest
	}

	return s.repo.RunInTransaction(ctx, func(txCtx context.Context) error {
		exists, err := s.repo.FriendshipExists(txCtx, request.SenderID, request.ReceiverID)
		if err != nil {
			return err
		}

		if !exists {
			if err := s.repo.CreateFriendships(txCtx, request.SenderID, request.ReceiverID, s.now()); err != nil {
				return err
			}
		}

		if err := s.repo.DeleteFriendRequestByID(txCtx, request.ID); errors.Is(err, mongo.ErrNoDocuments) {
			return ErrFriendRequestNotFound
		} else if err != nil {
			return err
		}

		return nil
	})
}

func (s *Service) DeclineFriendRequest(ctx context.Context, actorID, requestID string) error {
	actorID = strings.TrimSpace(actorID)
	requestID = strings.TrimSpace(requestID)

	if actorID == "" {
		return ErrInvalidActorID
	}
	if requestID == "" {
		return ErrInvalidRequestID
	}

	request, err := s.repo.GetFriendRequestByID(ctx, requestID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrFriendRequestNotFound
	}
	if err != nil {
		return err
	}
	if request.ReceiverID != actorID {
		return ErrUnauthorizedFriendRequest
	}

	if err := s.repo.DeleteFriendRequestByID(ctx, request.ID); errors.Is(err, mongo.ErrNoDocuments) {
		return ErrFriendRequestNotFound
	} else {
		return err
	}
}

func (s *Service) RemoveFriend(ctx context.Context, actorID, friendID string) error {
	actorID = strings.TrimSpace(actorID)
	friendID = strings.TrimSpace(friendID)

	if actorID == "" {
		return ErrInvalidActorID
	}
	if friendID == "" {
		return ErrInvalidFriendID
	}

	if err := s.repo.DeleteFriendships(ctx, actorID, friendID); errors.Is(err, mongo.ErrNoDocuments) {
		return ErrFriendNotFound
	} else {
		return err
	}
}

func (s *Service) ListFriends(ctx context.Context, userID string) ([]Friend, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidActorID
	}
	return s.repo.ListFriends(ctx, userID)
}

func (s *Service) ListPendingFriendRequests(ctx context.Context, userID string) ([]FriendRequest, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidActorID
	}
	return s.repo.ListPendingFriendRequests(ctx, userID)
}
