package main

import (
	"context"
	"log"
	"net"
	"time"

	auth "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal"
	authpb "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal/pb/auth"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

func main() {
	mongoDB, err := NewMongoClient("mongodb://localhost:27019", "userdb")
	if err != nil {
		log.Fatalf("failed to connecting to mongo database: %v", err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServiceServer(grpcServer, NewServer(auth.NewService(auth.NewMongoRepository(mongoDB))))

	log.Println("AuthService rodando na porta 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

func NewMongoClient(URI, dbName string) (*mongo.Database, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(URI))
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
