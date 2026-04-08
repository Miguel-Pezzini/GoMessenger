package chat

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
	consumerGroupName = "chat-service"
	readBatchSize     = 10
	readBlockTimeout  = 5 * time.Second
	claimMinIdle      = 30 * time.Second
)

type Server struct {
	addr               string
	streamName         string
	channelName        string
	chatEventsChannel  string
	notificationStream string
	rdb                *redis.Client
	service            *Service
	publisher          audit.Publisher
}

func NewServer(addr, streamName, channelName, chatEventsChannel, notificationStream string, rdb *redis.Client, service *Service, publisher audit.Publisher) *Server {
	return &Server{
		addr:               addr,
		streamName:         streamName,
		channelName:        channelName,
		chatEventsChannel:  chatEventsChannel,
		notificationStream: notificationStream,
		rdb:                rdb,
		service:            service,
		publisher:          publisher,
	}
}

func (s *Server) Start() error {
	ctx := context.Background()

	if err := s.ensureConsumerGroup(ctx); err != nil {
		return err
	}

	go func() {
		if err := s.subscribeChatEvents(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Println("failed to subscribe to chat interaction events:", err)
		}
	}()

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
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if strings.Contains(err.Error(), "NOGROUP") {
			return s.ensureConsumerGroup(ctx)
		}
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
		s.publishAudit(ctx, audit.Event{
			EventType:  "chat.decode.failed",
			Category:   audit.CategoryError,
			Service:    "chat",
			EntityType: "message",
			Status:     audit.StatusFailure,
			Message:    "chat stream message decode failed",
			Metadata:   map[string]any{"stream_id": msg.ID, "error": err.Error()},
		})
		if ackErr := s.ackMessage(ctx, msg.ID); ackErr != nil {
			return ackErr
		}
		return nil
	}

	req.StreamID = msg.ID

	messageResponse, err := s.service.Create(ctx, req)
	if err != nil {
		log.Printf("failed to persist message %s: %v", msg.ID, err)
		s.publishAudit(ctx, audit.Event{
			EventType:    "chat.persist.failed",
			Category:     audit.CategoryError,
			Service:      "chat",
			ActorUserID:  req.SenderID,
			TargetUserID: req.ReceiverID,
			EntityType:   "message",
			Status:       audit.StatusFailure,
			Message:      "chat message persistence failed",
			Metadata:     map[string]any{"stream_id": msg.ID, "error": err.Error()},
		})
		return nil
	}

	res, err := json.Marshal(messageResponse)
	if err != nil {
		log.Printf("failed to marshal response for %s: %v", msg.ID, err)
		return nil
	}

	if err := s.rdb.Publish(ctx, s.channelName, res).Err(); err != nil {
		log.Printf("failed to publish message %s to gateway channel: %v", msg.ID, err)
		s.publishAudit(ctx, audit.Event{
			EventType:    "chat.publish.failed",
			Category:     audit.CategoryError,
			Service:      "chat",
			ActorUserID:  req.SenderID,
			TargetUserID: req.ReceiverID,
			EntityType:   "message",
			EntityID:     messageResponse.Id,
			Status:       audit.StatusFailure,
			Message:      "chat persisted message publish failed",
			Metadata:     map[string]any{"stream_id": msg.ID, "error": err.Error()},
		})
		return nil
	}

	s.publishAudit(ctx, audit.Event{
		EventType:    "chat.message.persisted",
		Category:     audit.CategoryAudit,
		Service:      "chat",
		ActorUserID:  req.SenderID,
		TargetUserID: req.ReceiverID,
		EntityType:   "message",
		EntityID:     messageResponse.Id,
		Status:       audit.StatusSuccess,
		Message:      "chat message persisted",
		Metadata:     map[string]any{"stream_id": msg.ID},
	})

	if err := s.publishNotificationIntent(ctx, messageResponse); err != nil {
		log.Printf("failed to publish notification intent for %s: %v", msg.ID, err)
		return nil
	}

	return s.ackMessage(ctx, msg.ID)
}

func (s *Server) subscribeChatEvents(ctx context.Context) error {
	if s.chatEventsChannel == "" {
		return nil
	}

	pubsub := s.rdb.Subscribe(ctx, s.chatEventsChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := s.processInteractionPayload(ctx, msg.Payload); err != nil {
				log.Printf("failed to process chat interaction event: %v", err)
			}
		}
	}
}

func (s *Server) processInteractionPayload(ctx context.Context, payload string) error {
	var event InteractionEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return err
	}

	return s.processInteractionEvent(ctx, event)
}

func (s *Server) processInteractionEvent(ctx context.Context, event InteractionEvent) error {
	status := viewedStatusForInteractionEvent(event)
	if status == "" {
		return nil
	}

	message, err := s.service.UpdateViewedStatus(ctx, event.MessageID, event.ActorUserID, status)
	if err != nil {
		s.publishAudit(ctx, audit.Event{
			EventType:   "chat.viewed_status.update_failed",
			Category:    audit.CategoryError,
			Service:     "chat",
			ActorUserID: event.ActorUserID,
			EntityType:  "message",
			EntityID:    event.MessageID,
			Status:      audit.StatusFailure,
			Message:     "chat viewed status update failed",
			Metadata: map[string]any{
				"event_type":     event.Type,
				"viewed_status":  status,
				"target_user_id": event.TargetUserID,
				"error":          err.Error(),
			},
		})
		return err
	}

	s.publishAudit(ctx, audit.Event{
		EventType:    "chat.viewed_status.updated",
		Category:     audit.CategoryAudit,
		Service:      "chat",
		ActorUserID:  event.ActorUserID,
		TargetUserID: event.TargetUserID,
		EntityType:   "message",
		EntityID:     message.Id,
		Status:       audit.StatusSuccess,
		Message:      "chat viewed status updated",
		Metadata: map[string]any{
			"event_type":    event.Type,
			"viewed_status": message.ViewedStatus,
		},
	})

	return nil
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

func decodeMessage(msg redis.XMessage) (MessageRequest, error) {
	rawData, ok := msg.Values["payload"].(string)
	if !ok {
		return MessageRequest{}, errors.New("invalid message format, missing payload")
	}

	var req MessageRequest
	if err := json.Unmarshal([]byte(rawData), &req); err != nil {
		return MessageRequest{}, err
	}

	return req, nil
}

func (s *Server) publishAudit(ctx context.Context, event audit.Event) {
	if s.publisher == nil {
		return
	}
	_ = s.publisher.Publish(ctx, event)
}

type messageNotificationIntent struct {
	Type       string `json:"type"`
	EventID    string `json:"event_id"`
	MessageID  string `json:"message_id"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp,omitempty"`
	OccurredAt string `json:"occurred_at"`
}

func (s *Server) publishNotificationIntent(ctx context.Context, message *MessageResponse) error {
	if s.notificationStream == "" || message == nil {
		return nil
	}

	intent := messageNotificationIntent{
		Type:       "message_received",
		EventID:    message.Id,
		MessageID:  message.Id,
		SenderID:   message.SenderID,
		ReceiverID: message.ReceiverID,
		Content:    message.Content,
		Timestamp:  message.Timestamp,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(intent)
	if err != nil {
		return err
	}

	_, err = s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: s.notificationStream,
		Values: map[string]any{"payload": string(payload)},
	}).Result()
	return err
}

func viewedStatusForInteractionEvent(event InteractionEvent) string {
	if event.ViewedStatus != "" {
		return NormalizeViewedStatus(event.ViewedStatus)
	}

	switch event.Type {
	case "message_delivered":
		return ViewedStatusDelivered
	case "message_seen":
		return ViewedStatusSeen
	default:
		return ""
	}
}
