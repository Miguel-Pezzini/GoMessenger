package main

import (
	"context"
	"log"

	redisutil "github.com/Miguel-Pezzini/GoMessenger/services/presence_service/internal/redis"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	addr        string
	rdb         *redis.Client
	redisConfig *redisutil.RedisConfig
}

func NewServer(addr string) *Server {
	rdb, err := redisutil.NewRedisClient()
	if err != nil {
		log.Fatal("error connecting with redis", err)
	}
	redisConfig := redisutil.LoadRedisConfig()
	return &Server{addr: addr, rdb: rdb, redisConfig: redisConfig}
}

func (s *Server) Start() error {

	ctx := context.Background()
	service := chat.NewService(chat.NewMongoRepository(s.mongo))

	streamName := s.redisConfig.StreamChat

	return nil
}
