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

func (s *Server) SendFriendRequest(ctx context.Context, req *friendspb.SendFriendRequestRequest) (*friendspb.FriendRequestResponse, error) {
	request, err := s.service.SendFriendRequest(ctx, req.SenderId, req.ReceiverId)
	if err != nil {
		return nil, mapError(err)
	}
	return mapFriendRequestResponse(request), nil
}

func (s *Server) AcceptFriendRequest(ctx context.Context, req *friendspb.AcceptFriendRequestRequest) (*friendspb.ActionResponse, error) {
	if err := s.service.AcceptFriendRequest(ctx, req.ActorId, req.RequestId); err != nil {
		return nil, mapError(err)
	}
	return &friendspb.ActionResponse{Success: true}, nil
}

func (s *Server) DeclineFriendRequest(ctx context.Context, req *friendspb.DeclineFriendRequestRequest) (*friendspb.ActionResponse, error) {
	if err := s.service.DeclineFriendRequest(ctx, req.ActorId, req.RequestId); err != nil {
		return nil, mapError(err)
	}
	return &friendspb.ActionResponse{Success: true}, nil
}

func (s *Server) RemoveFriend(ctx context.Context, req *friendspb.RemoveFriendRequest) (*friendspb.ActionResponse, error) {
	if err := s.service.RemoveFriend(ctx, req.ActorId, req.FriendId); err != nil {
		return nil, mapError(err)
	}
	return &friendspb.ActionResponse{Success: true}, nil
}

func (s *Server) ListFriends(ctx context.Context, req *friendspb.ListFriendsRequest) (*friendspb.ListFriendsResponse, error) {
	friendsList, err := s.service.ListFriends(ctx, req.UserId)
	if err != nil {
		return nil, mapError(err)
	}

	response := &friendspb.ListFriendsResponse{Friends: make([]*friendspb.FriendResponse, 0, len(friendsList))}
	for _, friend := range friendsList {
		response.Friends = append(response.Friends, mapFriendResponse(friend))
	}
	return response, nil
}

func (s *Server) ListPendingFriendRequests(ctx context.Context, req *friendspb.ListPendingFriendRequestsRequest) (*friendspb.ListPendingFriendRequestsResponse, error) {
	requests, err := s.service.ListPendingFriendRequests(ctx, req.UserId)
	if err != nil {
		return nil, mapError(err)
	}

	response := &friendspb.ListPendingFriendRequestsResponse{Requests: make([]*friendspb.FriendRequestResponse, 0, len(requests))}
	for _, request := range requests {
		response.Requests = append(response.Requests, mapFriendRequestResponse(request))
	}
	return response, nil
}

func mapFriendResponse(friend domain.Friend) *friendspb.FriendResponse {
	return &friendspb.FriendResponse{
		Id:        friend.ID,
		UserId:    friend.UserID,
		FriendId:  friend.FriendID,
		CreatedAt: friend.CreatedAt.UTC().Format(timeRFC3339),
	}
}

func mapFriendRequestResponse(request domain.FriendRequest) *friendspb.FriendRequestResponse {
	return &friendspb.FriendRequestResponse{
		Id:         request.ID,
		SenderId:   request.SenderID,
		ReceiverId: request.ReceiverID,
		CreatedAt:  request.CreatedAt.UTC().Format(timeRFC3339),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidActorID),
		errors.Is(err, domain.ErrInvalidReceiverID),
		errors.Is(err, domain.ErrInvalidFriendID),
		errors.Is(err, domain.ErrInvalidRequestID),
		errors.Is(err, domain.ErrCannotSendRequestToYourself):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrAlreadyFriends), errors.Is(err, domain.ErrFriendRequestAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrFriendRequestNotFound), errors.Is(err, domain.ErrFriendNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrUnauthorizedFriendRequest):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
