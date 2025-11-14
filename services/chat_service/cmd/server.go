package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	chat "github.com/Miguel-Pezzini/real_time_chat/chat_service/internal"
	mongoutils "github.com/Miguel-Pezzini/real_time_chat/chat_service/internal/mongo"
	redisutil "github.com/Miguel-Pezzini/real_time_chat/chat_service/internal/redis"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

type Server struct {
	addr        string
	rdb         *redis.Client
	mongo       *mongo.Database
	redisConfig *redisutil.RedisConfig
}

func NewServer(addr string) *Server {
	mongo, err := mongoutils.NewMongoClient("mongodb://localhost:27018", "chatdb")
	if err != nil {
		log.Fatalf("failed to connecting to chat database: %v", err)
	}
	rdb, err := redisutil.NewRedisClient()
	if err != nil {
		log.Fatal("error connecting with redis", err)
	}
	redisConfig := redisutil.LoadRedisConfig()
	return &Server{addr: addr, rdb: rdb, mongo: mongo, redisConfig: redisConfig}
}

func (s *Server) Start() error {

	ctx := context.Background()
	service := chat.NewService(chat.NewMongoRepository(s.mongo))

	streamName := s.redisConfig.StreamChat

	func() {
		for {
			streams, err := s.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{streamName, "0"},
				Block:   5 * time.Second,
				Count:   10,
			}).Result()
			if err != nil {
				log.Println("XRead failed:", err)
				time.Sleep(time.Second)
				continue
			}

			for _, st := range streams {
				for _, msg := range st.Messages {

					rawData, ok := msg.Values["data"].(string)
					if !ok {
						log.Println("invalid message format, missing 'data'")
						_ = s.rdb.XDel(ctx, streamName, msg.ID).Err()
						continue
					}

					var req chat.MessageRequest
					if err := json.Unmarshal([]byte(rawData), &req); err != nil {
						log.Println("failed to unmarshal message request:", err)
						_ = s.rdb.XDel(ctx, streamName, msg.ID).Err()
						continue
					}
					messageResponse, err := service.Create(ctx, req)
					if err != nil {
						log.Println("failed to persist message:", err)
						continue
					}
					res, err := json.Marshal(messageResponse)
					if err != nil {
						log.Println("failed to marshal response:", err)
						continue
					}

					channel := os.Getenv("REDIS_CHANNEL_CHAT")
					if err := s.rdb.Publish(ctx, channel, res).Err(); err != nil {
						log.Println("failed to publish to gateway channel:", err)
						continue
					}

					if err := s.rdb.XDel(ctx, streamName, msg.ID).Err(); err != nil {
						log.Println("failed to delete processed stream entry:", err)
					}
				}
			}
		}
	}()

	return nil
}
