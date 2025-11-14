package main

import (
	"context"

	auth "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal"
	authpb "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal/pb/auth"
)

type Server struct {
	authpb.UnimplementedAuthServiceServer
	service *auth.Service
}

func NewServer() (*Server, error) {
	mongoDB, err := NewMongoClient("mongodb://localhost:27019", "userdb")
	if err != nil {
		return nil, err
	}
	repo := auth.NewMongoRepository(mongoDB)
	svc := auth.NewService(repo)
	return &Server{service: svc}, nil
}

func (s *Server) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	return s.service.Register(req)
}

func (s *Server) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	return s.service.Authenticate(req)
}
