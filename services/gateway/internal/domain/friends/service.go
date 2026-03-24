package friends

import (
	"context"

	friendspb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/friendspb"
	"google.golang.org/grpc"
)

type ServiceClient interface {
	SendFriendRequest(ctx context.Context, in *friendspb.SendFriendRequestRequest, opts ...grpc.CallOption) (*friendspb.FriendRequestResponse, error)
	AcceptFriendRequest(ctx context.Context, in *friendspb.AcceptFriendRequestRequest, opts ...grpc.CallOption) (*friendspb.ActionResponse, error)
	DeclineFriendRequest(ctx context.Context, in *friendspb.DeclineFriendRequestRequest, opts ...grpc.CallOption) (*friendspb.ActionResponse, error)
	RemoveFriend(ctx context.Context, in *friendspb.RemoveFriendRequest, opts ...grpc.CallOption) (*friendspb.ActionResponse, error)
	ListFriends(ctx context.Context, in *friendspb.ListFriendsRequest, opts ...grpc.CallOption) (*friendspb.ListFriendsResponse, error)
	ListPendingFriendRequests(ctx context.Context, in *friendspb.ListPendingFriendRequestsRequest, opts ...grpc.CallOption) (*friendspb.ListPendingFriendRequestsResponse, error)
}

type Service struct {
	client ServiceClient
}

func NewService(client ServiceClient) *Service {
	return &Service{client: client}
}

func (s *Service) SendFriendRequest(ctx context.Context, actorID string, req SendFriendRequestRequest) (*FriendRequest, error) {
	res, err := s.client.SendFriendRequest(ctx, &friendspb.SendFriendRequestRequest{
		SenderId:   actorID,
		ReceiverId: req.ReceiverID,
	})
	if err != nil {
		return nil, err
	}
	return mapFriendRequest(res), nil
}

func (s *Service) AcceptFriendRequest(ctx context.Context, actorID, requestID string) error {
	_, err := s.client.AcceptFriendRequest(ctx, &friendspb.AcceptFriendRequestRequest{
		ActorId:   actorID,
		RequestId: requestID,
	})
	return err
}

func (s *Service) DeclineFriendRequest(ctx context.Context, actorID, requestID string) error {
	_, err := s.client.DeclineFriendRequest(ctx, &friendspb.DeclineFriendRequestRequest{
		ActorId:   actorID,
		RequestId: requestID,
	})
	return err
}

func (s *Service) RemoveFriend(ctx context.Context, actorID, friendID string) error {
	_, err := s.client.RemoveFriend(ctx, &friendspb.RemoveFriendRequest{
		ActorId:  actorID,
		FriendId: friendID,
	})
	return err
}

func (s *Service) ListFriends(ctx context.Context, actorID string) ([]*Friend, error) {
	res, err := s.client.ListFriends(ctx, &friendspb.ListFriendsRequest{UserId: actorID})
	if err != nil {
		return nil, err
	}

	friends := make([]*Friend, 0, len(res.Friends))
	for _, item := range res.Friends {
		friends = append(friends, mapFriend(item))
	}
	return friends, nil
}

func (s *Service) ListPendingFriendRequests(ctx context.Context, actorID string) ([]*FriendRequest, error) {
	res, err := s.client.ListPendingFriendRequests(ctx, &friendspb.ListPendingFriendRequestsRequest{UserId: actorID})
	if err != nil {
		return nil, err
	}

	requests := make([]*FriendRequest, 0, len(res.Requests))
	for _, item := range res.Requests {
		requests = append(requests, mapFriendRequest(item))
	}
	return requests, nil
}

func mapFriend(pb *friendspb.FriendResponse) *Friend {
	return &Friend{
		ID:        pb.Id,
		UserID:    pb.UserId,
		FriendID:  pb.FriendId,
		CreatedAt: pb.CreatedAt,
	}
}

func mapFriendRequest(pb *friendspb.FriendRequestResponse) *FriendRequest {
	return &FriendRequest{
		ID:         pb.Id,
		SenderID:   pb.SenderId,
		ReceiverID: pb.ReceiverId,
		CreatedAt:  pb.CreatedAt,
	}
}
