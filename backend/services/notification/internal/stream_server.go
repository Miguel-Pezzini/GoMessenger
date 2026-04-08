package notification

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	readBatchSize    = 10
	readBlockTimeout = 5 * time.Second
	claimMinIdle     = 30 * time.Second
)

type StreamServer struct {
	rdb           *redis.Client
	service       *Service
	friendStream  string
	friendGroup   string
	messageStream string
	messageGroup  string
	consumerName  string
}

func NewStreamServer(rdb *redis.Client, service *Service, friendStream, friendGroup, messageStream, messageGroup, consumerName string) *StreamServer {
	return &StreamServer{
		rdb:           rdb,
		service:       service,
		friendStream:  friendStream,
		friendGroup:   friendGroup,
		messageStream: messageStream,
		messageGroup:  messageGroup,
		consumerName:  consumerName,
	}
}

func (s *StreamServer) Start(ctx context.Context) error {
	if err := s.ensureConsumerGroup(ctx, s.friendStream, s.friendGroup); err != nil {
		return err
	}
	if err := s.ensureConsumerGroup(ctx, s.messageStream, s.messageGroup); err != nil {
		return err
	}

	errCh := make(chan error, 2)
	go s.runLoop(ctx, errCh, s.friendStream, s.friendGroup, s.processFriendMessage)
	go s.runLoop(ctx, errCh, s.messageStream, s.messageGroup, s.processMessageNotification)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
}

func (s *StreamServer) runLoop(ctx context.Context, errCh chan<- error, streamName, groupName string, handler func(context.Context, string, redis.XMessage) error) {
	for {
		if err := s.processClaimedMessages(ctx, streamName, groupName, handler); err != nil {
			if errors.Is(err, context.Canceled) {
				errCh <- err
				return
			}
			log.Printf("notification: failed to process claimed messages for %s: %v", streamName, err)
			time.Sleep(time.Second)
			continue
		}

		if err := s.processNewMessages(ctx, streamName, groupName, handler); err != nil {
			if errors.Is(err, context.Canceled) {
				errCh <- err
				return
			}
			log.Printf("notification: failed to process new messages for %s: %v", streamName, err)
			time.Sleep(time.Second)
		}
	}
}

func (s *StreamServer) ensureConsumerGroup(ctx context.Context, streamName, groupName string) error {
	err := s.rdb.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (s *StreamServer) processClaimedMessages(ctx context.Context, streamName, groupName string, handler func(context.Context, string, redis.XMessage) error) error {
	messages, _, err := s.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   streamName,
		Group:    groupName,
		Consumer: s.consumer(),
		MinIdle:  claimMinIdle,
		Start:    "0-0",
		Count:    readBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if strings.Contains(err.Error(), "NOGROUP") {
			return s.ensureConsumerGroup(ctx, streamName, groupName)
		}
		return err
	}

	for _, msg := range messages {
		if err := handler(ctx, groupName, msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *StreamServer) processNewMessages(ctx context.Context, streamName, groupName string, handler func(context.Context, string, redis.XMessage) error) error {
	streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: s.consumer(),
		Streams:  []string{streamName, ">"},
		Block:    readBlockTimeout,
		Count:    readBatchSize,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if strings.Contains(err.Error(), "NOGROUP") {
			return s.ensureConsumerGroup(ctx, streamName, groupName)
		}
		return err
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			if err := handler(ctx, groupName, msg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *StreamServer) processFriendMessage(ctx context.Context, groupName string, msg redis.XMessage) error {
	intent, err := decodeFriendRequestIntent(msg)
	if err != nil {
		log.Printf("notification: failed to decode friend request intent %s: %v", msg.ID, err)
		return s.ackMessage(ctx, s.friendStream, groupName, msg.ID)
	}

	if err := s.service.HandleFriendRequestIntent(ctx, intent); err != nil {
		log.Printf("notification: failed to handle friend request intent %s: %v", msg.ID, err)
		return s.ackMessage(ctx, s.friendStream, groupName, msg.ID)
	}

	return s.ackMessage(ctx, s.friendStream, groupName, msg.ID)
}

func (s *StreamServer) processMessageNotification(ctx context.Context, groupName string, msg redis.XMessage) error {
	intent, err := decodeMessageIntent(msg)
	if err != nil {
		log.Printf("notification: failed to decode message intent %s: %v", msg.ID, err)
		return s.ackMessage(ctx, s.messageStream, groupName, msg.ID)
	}

	_, err = s.service.HandleMessageIntent(ctx, intent)
	if err != nil {
		log.Printf("notification: failed to handle message intent %s: %v", msg.ID, err)
		return nil
	}

	return s.ackMessage(ctx, s.messageStream, groupName, msg.ID)
}

func (s *StreamServer) ackMessage(ctx context.Context, streamName, groupName, messageID string) error {
	return s.rdb.XAck(ctx, streamName, groupName, messageID).Err()
}

func (s *StreamServer) consumer() string {
	if s.consumerName != "" {
		return s.consumerName
	}
	return "notification-consumer"
}

func decodeFriendRequestIntent(msg redis.XMessage) (FriendRequestIntent, error) {
	rawData, ok := msg.Values["payload"].(string)
	if !ok {
		return FriendRequestIntent{}, errors.New("invalid message format, missing payload")
	}

	var intent FriendRequestIntent
	if err := json.Unmarshal([]byte(rawData), &intent); err != nil {
		return FriendRequestIntent{}, err
	}
	return intent, nil
}

func decodeMessageIntent(msg redis.XMessage) (MessageIntent, error) {
	rawData, ok := msg.Values["payload"].(string)
	if !ok {
		return MessageIntent{}, errors.New("invalid message format, missing payload")
	}

	var intent MessageIntent
	if err := json.Unmarshal([]byte(rawData), &intent); err != nil {
		return MessageIntent{}, err
	}
	return intent, nil
}
