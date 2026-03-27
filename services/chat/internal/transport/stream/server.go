package stream

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	consumerGroupName = "chat-service"
	readBatchSize     = 10
	readBlockTimeout  = 5 * time.Second
	claimMinIdle      = 30 * time.Second
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

	if err := s.ensureConsumerGroup(ctx); err != nil {
		return err
	}

	go func() {
		for {
			if err := s.processClaimedMessages(ctx); err != nil {
				log.Println("failed to process claimed messages:", err)
				time.Sleep(time.Second)
				continue
			}

			if err := s.processNewMessages(ctx); err != nil {
				log.Println("failed to process new stream messages:", err)
				time.Sleep(time.Second)
			}
		}
	}()

	return nil
}

func (s *Server) ensureConsumerGroup(ctx context.Context) error {
	err := s.rdb.XGroupCreateMkStream(ctx, s.streamName, consumerGroupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (s *Server) processClaimedMessages(ctx context.Context) error {
	messages, _, err := s.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   s.streamName,
		Group:    consumerGroupName,
		Consumer: s.consumerName(),
		MinIdle:  claimMinIdle,
		Start:    "0-0",
		Count:    readBatchSize,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	return s.processMessages(ctx, messages)
}

func (s *Server) processNewMessages(ctx context.Context) error {
	streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroupName,
		Consumer: s.consumerName(),
		Streams:  []string{s.streamName, ">"},
		Block:    readBlockTimeout,
		Count:    readBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return err
	}

	for _, stream := range streams {
		if err := s.processMessages(ctx, stream.Messages); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) processMessages(ctx context.Context, messages []redis.XMessage) error {
	for _, msg := range messages {
		if err := s.processMessage(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) processMessage(ctx context.Context, msg redis.XMessage) error {
	req, err := decodeMessage(msg)
	if err != nil {
		log.Printf("failed to decode stream message %s: %v", msg.ID, err)
		if ackErr := s.ackMessage(ctx, msg.ID); ackErr != nil {
			return ackErr
		}
		return nil
	}

	req.StreamID = msg.ID

	messageResponse, err := s.service.Create(ctx, req)
	if err != nil {
		log.Printf("failed to persist message %s: %v", msg.ID, err)
		return nil
	}

	res, err := json.Marshal(messageResponse)
	if err != nil {
		log.Printf("failed to marshal response for %s: %v", msg.ID, err)
		return nil
	}

	if err := s.rdb.Publish(ctx, s.channelName, res).Err(); err != nil {
		log.Printf("failed to publish message %s to gateway channel: %v", msg.ID, err)
		return nil
	}

	return s.ackMessage(ctx, msg.ID)
}

func (s *Server) ackMessage(ctx context.Context, messageID string) error {
	return s.rdb.XAck(ctx, s.streamName, consumerGroupName, messageID).Err()
}

func (s *Server) consumerName() string {
	if s.addr != "" {
		return s.addr
	}
	return "chat-consumer"
}

func decodeMessage(msg redis.XMessage) (domain.MessageRequest, error) {
	rawData, ok := msg.Values["payload"].(string)
	if !ok {
		return domain.MessageRequest{}, errors.New("invalid message format, missing payload")
	}

	var req domain.MessageRequest
	if err := json.Unmarshal([]byte(rawData), &req); err != nil {
		return domain.MessageRequest{}, err
	}

	return req, nil
}
