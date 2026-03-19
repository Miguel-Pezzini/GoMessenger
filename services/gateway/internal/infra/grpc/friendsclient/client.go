package friendsclient

import (
	"context"
	"time"

	friendspb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/friendspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func New(address string) (friendspb.FriendsServiceClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(friendspb.JSONCodec())),
	)
	if err != nil {
		return nil, err
	}

	return friendspb.NewFriendsServiceClient(conn), nil
}
