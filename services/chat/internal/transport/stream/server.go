package stream

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/domain"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	addr        string
	streamName  string
	channelName string
	rdb         *redis.Client
	service     *domain.Service
}

func NewServer(addr, streamName, channelName string, rdb *redis.Client, service *domain.Service) *Server {
	return &Server{
		addr:        addr,
		streamName:  streamName,
		channelName: channelName,
		rdb:         rdb,
		service:     service,
	}
}

func (s *Server) Start() error {
	ctx := context.Background()

	func() {
		for {
			streams, err := s.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{s.streamName, "0"},
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

					rawData, ok := msg.Values["payload"].(string)
					if !ok {
						log.Println("invalid message format, missing 'payload'")
						_ = s.rdb.XDel(ctx, s.streamName, msg.ID).Err()
						continue
					}

					var req domain.MessageRequest
					if err := json.Unmarshal([]byte(rawData), &req); err != nil {
						log.Println("failed to unmarshal message request:", err)
						_ = s.rdb.XDel(ctx, s.streamName, msg.ID).Err()
						continue
					}
					messageResponse, err := s.service.Create(ctx, req)
					if err != nil {
						log.Println("failed to persist message:", err)
						continue
					}
					res, err := json.Marshal(messageResponse)
					if err != nil {
						log.Println("failed to marshal response:", err)
						continue
					}

					if err := s.rdb.Publish(ctx, s.channelName, res).Err(); err != nil {
						log.Println("failed to publish to gateway channel:", err)
						continue
					}

					if err := s.rdb.XDel(ctx, s.streamName, msg.ID).Err(); err != nil {
						log.Println("failed to delete processed stream entry:", err)
					}
				}
			}
		}
	}()

	return nil
}
