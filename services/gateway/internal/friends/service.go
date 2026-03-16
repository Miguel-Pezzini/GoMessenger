package friends

import (
	"context"

	friendspb "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/pb/friends"
)

type Service struct {
	client friendspb.FriendsServiceClient
}

func NewService(client friendspb.FriendsServiceClient) *Service {
	return &Service{client: client}
}

func (s *Service) Create(ctx context.Context, ownerID string, req CreateFriendRequest) (*Friend, error) {
	res, err := s.client.CreateFriend(ctx, &friendspb.CreateFriendRequest{
		OwnerId:  ownerID,
		Username: req.Username,
		Name:     req.Name,
	})
	if err != nil {
		return nil, err
	}
	return mapFriend(res), nil
}

func (s *Service) List(ctx context.Context, ownerID string) ([]*Friend, error) {
	res, err := s.client.ListFriends(ctx, &friendspb.ListFriendsRequest{OwnerId: ownerID})
	if err != nil {
		return nil, err
	}

	friends := make([]*Friend, 0, len(res.GetFriends()))
	for _, item := range res.GetFriends() {
		friends = append(friends, mapFriend(item))
	}
	return friends, nil
}

func (s *Service) GetByID(ctx context.Context, ownerID, id string) (*Friend, error) {
	res, err := s.client.GetFriend(ctx, &friendspb.GetFriendRequest{OwnerId: ownerID, Id: id})
	if err != nil {
		return nil, err
	}
	return mapFriend(res), nil
}

func (s *Service) Update(ctx context.Context, ownerID, id string, req UpdateFriendRequest) (*Friend, error) {
	res, err := s.client.UpdateFriend(ctx, &friendspb.UpdateFriendRequest{
		OwnerId:  ownerID,
		Id:       id,
		Username: req.Username,
		Name:     req.Name,
	})
	if err != nil {
		return nil, err
	}
	return mapFriend(res), nil
}

func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	_, err := s.client.DeleteFriend(ctx, &friendspb.DeleteFriendRequest{OwnerId: ownerID, Id: id})
	return err
}

func mapFriend(pb *friendspb.FriendResponse) *Friend {
	return &Friend{
		ID:        pb.GetId(),
		OwnerID:   pb.GetOwnerId(),
		Username:  pb.GetUsername(),
		Name:      pb.GetName(),
		CreatedAt: pb.GetCreatedAt(),
		UpdatedAt: pb.GetUpdatedAt(),
	}
}
