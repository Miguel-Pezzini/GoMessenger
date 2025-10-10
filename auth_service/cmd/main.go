package main

import (
	"log"
	"net"

	auth "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal"
	authpb "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal/pb/auth"
	db "github.com/Miguel-Pezzini/real_time_chat/pkg/db"
	"google.golang.org/grpc"
)

func main() {
	mongoDB, err := db.NewMongoClient("mongodb://localhost:27017", "userdb")
	if err != nil {
		log.Fatal(err)
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
