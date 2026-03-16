package grpc

import (
	"context"
	"errors"
	"fmt"

	friendspb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/friendspb"
	"github.com/Miguel-Pezzini/GoMessenger/services/friends/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	friendspb.UnimplementedFriendsServiceServer
	service *domain.Service
}

func NewServer(service *domain.Service) *Server {
	return &Server{service: service}
}

func (s *Server) CreateFriend(ctx context.Context, req *friendspb.CreateFriendRequest) (*friendspb.FriendResponse, error) {
	friend, err := s.service.Create(ctx, req.GetOwnerId(), req.GetUsername(), req.GetName())
	if err != nil {
		return nil, mapError(err)
	}
	return mapFriendResponse(friend), nil
}

func (s *Server) GetFriend(ctx context.Context, req *friendspb.GetFriendRequest) (*friendspb.FriendResponse, error) {
	friend, err := s.service.GetByID(ctx, req.GetOwnerId(), req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return mapFriendResponse(friend), nil
}

func (s *Server) ListFriends(ctx context.Context, req *friendspb.ListFriendsRequest) (*friendspb.ListFriendsResponse, error) {
	friendsList, err := s.service.ListByOwner(ctx, req.GetOwnerId())
	if err != nil {
		return nil, mapError(err)
	}

	response := &friendspb.ListFriendsResponse{Friends: make([]*friendspb.FriendResponse, 0, len(friendsList))}
	for _, friend := range friendsList {
		response.Friends = append(response.Friends, mapFriendResponse(friend))
	}
	return response, nil
}

func (s *Server) UpdateFriend(ctx context.Context, req *friendspb.UpdateFriendRequest) (*friendspb.FriendResponse, error) {
	friend, err := s.service.Update(ctx, req.GetOwnerId(), req.GetId(), req.GetUsername(), req.GetName())
	if err != nil {
		return nil, mapError(err)
	}
	return mapFriendResponse(friend), nil
}

func (s *Server) DeleteFriend(ctx context.Context, req *friendspb.DeleteFriendRequest) (*friendspb.DeleteFriendResponse, error) {
	if err := s.service.Delete(ctx, req.GetOwnerId(), req.GetId()); err != nil {
		return nil, mapError(err)
	}
	return &friendspb.DeleteFriendResponse{Deleted: true}, nil
}

func mapFriendResponse(friend domain.Friend) *friendspb.FriendResponse {
	return &friendspb.FriendResponse{
		Id:        friend.ID,
		OwnerId:   friend.OwnerID,
		Username:  friend.Username,
		Name:      friend.Name,
		CreatedAt: friend.CreatedAt.UTC().Format(timeRFC3339),
		UpdatedAt: friend.UpdatedAt.UTC().Format(timeRFC3339),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidOwnerID), errors.Is(err, domain.ErrInvalidUsername):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrFriendNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
