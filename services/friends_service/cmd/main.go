package main

import (
	"context"
	"log"
	"net"
	"time"

	friendspb "github.com/Miguel-Pezzini/GoMessenger/services/friends_service/internal/pb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

func main() {
	srv, err := NewServer()
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	friendspb.RegisterFriendsServiceServer(grpcServer, srv)

	log.Println("FriendsService running on port 50052")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

func NewMongoClient(uri, dbName string) (*mongo.Database, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client.Database(dbName), nil
}
