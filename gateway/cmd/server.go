package main

import (
	"net/http"
	"real_time_chat/internal/auth"
	"real_time_chat/internal/websocket"

	"go.mongodb.org/mongo-driver/mongo"
)

type Server struct {
	addr string
	db   *mongo.Database
}

func NewServer(addr string, db *mongo.Database) *Server {
	return &Server{addr: addr, db: db}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.Handle("GET /ws", auth.JWTMiddleware(http.HandlerFunc(websocket.WsHandler)))

	authHandler := auth.NewHandler(auth.NewService(auth.NewRepository(s.db)))

	mux.Handle("POST /auth/login", http.HandlerFunc(authHandler.LoginHandler))
	mux.Handle("POST /auth/register", http.HandlerFunc(authHandler.RegisterHandler))
	return http.ListenAndServe(s.addr, mux)
}
