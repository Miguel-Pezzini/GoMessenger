package main

import (
	"net/http"

	"github.com/Miguel-Pezzini/real_time_chat/gateway/internal/auth"
	"github.com/Miguel-Pezzini/real_time_chat/gateway/internal/websocket"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type Server struct {
	addr string
	db   *mongo.Database
	rdb  *redis.Client
}

func NewServer(addr string, db *mongo.Database, rdb *redis.Client) *Server {
	return &Server{addr: addr, db: db, rdb: rdb}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	wsHandler := websocket.NewWsHandler(websocket.NewService(websocket.NewRedisRepository(s.rdb)))
	mux.Handle("GET /ws", auth.JWTMiddleware(http.HandlerFunc(wsHandler.HandleConnection)))

	authServiceClient, error := auth.NewAuthServiceClient("auth_service:50051")

	if error != nil {
		return error
	}

	authHandler := auth.NewHandler(auth.NewService(authServiceClient))
	mux.Handle("POST /auth/login", http.HandlerFunc(authHandler.LoginHandler))
	mux.Handle("POST /auth/register", http.HandlerFunc(authHandler.RegisterHandler))
	return http.ListenAndServe(s.addr, mux)
}
