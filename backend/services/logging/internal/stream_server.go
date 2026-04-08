package logging

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/redis/go-redis/v9"
)

const (
	consumerGroupName = "logging-service"
	readBatchSize     = 20
	readBlockTimeout  = 5 * time.Second
)

type Server struct {
	streamName string
	rdb        *redis.Client
	service    *Service
}

func NewServer(streamName string, rdb *redis.Client, service *Service) *Server {
	return &Server{
		streamName: streamName,
		rdb:        rdb,
		service:    service,
	}
}

func (s *Server) Start(ctx context.Context) {
	if err := s.ensureConsumerGroup(ctx); err != nil {
		log.Printf("logging: failed to ensure consumer group: %v", err)
		return
	}

	for {
		if err := s.processMessages(ctx); err != nil {
			log.Printf("logging: failed to process stream messages: %v", err)
			time.Sleep(time.Second)
		}
	}
}

func (s *Server) ensureConsumerGroup(ctx context.Context) error {
	err := s.rdb.XGroupCreateMkStream(ctx, s.streamName, consumerGroupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (s *Server) processMessages(ctx context.Context) error {
	streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroupName,
		Consumer: "logging-consumer",
		Streams:  []string{s.streamName, ">"},
		Block:    readBlockTimeout,
		Count:    readBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if strings.Contains(err.Error(), "NOGROUP") {
			return s.ensureConsumerGroup(ctx)
		}
		return err
	}

	for _, stream := range streams {
		for _, message := range stream.Messages {
			if err := s.processMessage(ctx, message); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Server) processMessage(ctx context.Context, msg redis.XMessage) error {
	event, err := decodeEvent(msg)
	if err != nil {
		log.Printf("logging: failed to decode audit event %s: %v", msg.ID, err)
		return s.ackMessage(ctx, msg.ID)
	}

	if _, err := s.service.Ingest(ctx, msg.ID, event); err != nil {
		log.Printf("logging: failed to ingest audit event %s: %v", msg.ID, err)
		return nil
	}

	return s.ackMessage(ctx, msg.ID)
}

func (s *Server) ackMessage(ctx context.Context, messageID string) error {
	return s.rdb.XAck(ctx, s.streamName, consumerGroupName, messageID).Err()
}

func decodeEvent(msg redis.XMessage) (audit.Event, error) {
	rawData, ok := msg.Values["payload"].(string)
	if !ok {
		return audit.Event{}, errors.New("invalid audit message format")
	}

	var event audit.Event
	if err := json.Unmarshal([]byte(rawData), &event); err != nil {
		return audit.Event{}, err
	}
	return event, nil
}
