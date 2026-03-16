package friendsclient

import (
	"context"
	"time"

	friendspb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/friendspb"
	"google.golang.org/grpc"
)

func New(address string) (friendspb.FriendsServiceClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	return friendspb.NewFriendsServiceClient(conn), nil
}
