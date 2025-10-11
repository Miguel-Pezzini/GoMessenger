package auth

import (
	"context"
	"time"

	authpb "github.com/Miguel-Pezzini/real_time_chat/gateway/internal/pb"
	"google.golang.org/grpc"
)

func NewAuthServiceClient(address string) (authpb.AuthServiceClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	return authpb.NewAuthServiceClient(conn), nil
}
