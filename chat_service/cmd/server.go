package main

import (
	"net/http"

	"github.com/Miguel-Pezzini/real_time_chat/gateway/internal/auth"
	authpb "github.com/Miguel-Pezzini/real_time_chat/gateway/internal/pb"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	addr string
	rdb  *redis.Client
}

func NewServer(addr string, rdb *redis.Client, auth_service authpb.AuthServiceClient) *Server {
	return &Server{addr: addr, rdb: rdb}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	wsHandler := websocket.NewWsHandler(websocket.NewService(websocket.NewRedisRepository(s.rdb)))
	mux.Handle("GET /ws", auth.JWTMiddleware(http.HandlerFunc(wsHandler.HandleConnection)))

	authHandler := auth.NewHandler(auth.NewService(s.auth_service))
	mux.Handle("POST /auth/login", http.HandlerFunc(authHandler.LoginHandler))
	mux.Handle("POST /auth/register", http.HandlerFunc(authHandler.RegisterHandler))
	return http.ListenAndServe(s.addr, mux)
}
