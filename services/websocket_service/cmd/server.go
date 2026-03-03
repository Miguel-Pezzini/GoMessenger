package main

import (
	"log"
	"net/http"

	redisutil "github.com/Miguel-Pezzini/GoMessenger/services/websocket_service/internal/redis"
	"github.com/Miguel-Pezzini/GoMessenger/services/websocket_service/internal/websocket"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	addr string
	rdb  *redis.Client
}

func NewServer(addr string) *Server {
	redisClient, err := redisutil.NewRedisClient()
	if err != nil {
		log.Fatal("error connecting with redis", err)
	}

	return &Server{addr: addr, rdb: redisClient}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	wsHandler := websocket.NewWsHandler(websocket.NewService(websocket.NewRedisRepository(s.rdb)))
	wsHandler.StartPubSubListener()

	mux.Handle("GET /ws", http.HandlerFunc(wsHandler.HandleConnection))
	return http.ListenAndServe(s.addr, mux)
}
