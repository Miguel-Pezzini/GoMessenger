package main

import (
	"log"
	"net/http"

	"github.com/Miguel-Pezzini/real_time_chat/gateway/internal/auth"
	authpb "github.com/Miguel-Pezzini/real_time_chat/gateway/internal/pb"
	redisutil "github.com/Miguel-Pezzini/real_time_chat/gateway/internal/redis"
	"github.com/Miguel-Pezzini/real_time_chat/gateway/internal/websocket"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	addr         string
	rdb          *redis.Client
	auth_service authpb.AuthServiceClient
}

func NewServer(addr, authAddr string) *Server {
	redisClient, err := redisutil.NewRedisClient()
	if err != nil {
		log.Fatal("error connecting with redis", err)
	}
	authService, err := auth.NewAuthServiceClient(authAddr)
	if err != nil {
		log.Fatal("error connecting with auth service", err)
	}
	log.Println("Gateway connected with Authentication Service")
	return &Server{addr: addr, rdb: redisClient, auth_service: authService}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	wsHandler := websocket.NewWsHandler(websocket.NewService(websocket.NewRedisRepository(s.rdb)))
	// mux.Handle("GET /ws", auth.JWTMiddleware(http.HandlerFunc(wsHandler.HandleConnection)))
	mux.Handle("GET /ws", auth.JWTMiddleware(http.HandlerFunc(wsHandler.HandleConnection)))

	authHandler := auth.NewHandler(auth.NewService(s.auth_service))
	mux.Handle("POST /auth/login", http.HandlerFunc(authHandler.LoginHandler))
	mux.Handle("POST /auth/register", http.HandlerFunc(authHandler.RegisterHandler))
	return http.ListenAndServe(s.addr, mux)
}
