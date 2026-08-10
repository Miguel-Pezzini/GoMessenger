package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/observability"
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
	mediaBinder        MediaBinder
	observer           *observability.Observer
}

func NewServer(addr, streamName, channelName, chatEventsChannel, notificationStream string, rdb *redis.Client, service *Service, publisher audit.Publisher, mediaBinders ...MediaBinder) *Server {
	server := &Server{
		addr:               addr,
		streamName:         streamName,
		channelName:        channelName,
		chatEventsChannel:  chatEventsChannel,
		notificationStream: notificationStream,
		rdb:                rdb,
		service:            service,
		publisher:          publisher,
	}
	if len(mediaBinders) > 0 {
		server.mediaBinder = mediaBinders[0]
	}
	return server
}

func (s *Server) SetObserver(observer *observability.Observer) {
	s.observer = observer
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

		s.observeStreamState(ctx)
	}
}

func (s *Server) ensureConsumerGroup(ctx context.Context) error {
	start := time.Now()
	err := s.rdb.XGroupCreateMkStream(ctx, s.streamName, consumerGroupName, "0").Err()
	result := resultLabel(err)
	if err != nil && strings.Contains(err.Error(), "BUSYGROUP") {
		result = "success"
	}
	s.observer.ObserveChatRedisOperation("xgroup_create", result, time.Since(start))
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (s *Server) processClaimedMessages(ctx context.Context) error {
	start := time.Now()
	messages, _, err := s.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   s.streamName,
		Group:    consumerGroupName,
		Consumer: s.consumerName(),
		MinIdle:  claimMinIdle,
		Start:    "0-0",
		Count:    readBatchSize,
	}).Result()
	s.observer.ObserveChatRedisOperation("xautoclaim", resultLabel(err), time.Since(start))
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
	start := time.Now()
	streams, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroupName,
		Consumer: s.consumerName(),
		Streams:  []string{s.streamName, ">"},
		Block:    readBlockTimeout,
		Count:    readBatchSize,
	}).Result()
	s.observer.ObserveChatRedisOperation("xreadgroup", resultLabel(err), time.Since(start))
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
	start := time.Now()
	age := observability.StreamMessageAge(msg.ID, start)
	result := "success"
	defer func() {
		s.observer.ObserveChatStreamMessage(result, age, time.Since(start))
	}()

	req, err := decodeMessage(msg)
	if err != nil {
		result = "decode_error"
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

	mongoStart := time.Now()
	messageResponse, err := s.service.Create(ctx, req)
	s.observer.ObserveChatMongoOperation("create_message", resultLabel(err), time.Since(mongoStart))
	if err != nil {
		result = "persist_error"
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

	if err := s.bindMessageAttachments(ctx, messageResponse); err != nil {
		result = "attachment_bind_error"
		log.Printf("failed to bind attachments for message %s: %v", msg.ID, err)
		s.publishAudit(ctx, audit.Event{
			EventType:    "chat.attachment_bind.failed",
			Category:     audit.CategoryError,
			Service:      "chat",
			ActorUserID:  req.SenderID,
			TargetUserID: req.ReceiverID,
			EntityType:   "message",
			EntityID:     messageResponse.Id,
			Status:       audit.StatusFailure,
			Message:      "chat message attachment binding failed",
			Metadata:     map[string]any{"stream_id": msg.ID, "error": err.Error()},
		})
		return nil
	}

	res, err := json.Marshal(messageResponse)
	if err != nil {
		result = "marshal_error"
		log.Printf("failed to marshal response for %s: %v", msg.ID, err)
		return nil
	}

	publishStart := time.Now()
	if err := s.rdb.Publish(ctx, s.channelName, res).Err(); err != nil {
		result = "publish_error"
		s.observer.ObserveChatRedisOperation("publish_persisted_message", "failure", time.Since(publishStart))
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
	s.observer.ObserveChatRedisOperation("publish_persisted_message", "success", time.Since(publishStart))

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
		result = "notification_error"
		log.Printf("failed to publish notification intent for %s: %v", msg.ID, err)
		return nil
	}

	return s.ackMessage(ctx, msg.ID)
}

func (s *Server) bindMessageAttachments(ctx context.Context, message *MessageResponse) error {
	if message == nil || len(message.Attachments) == 0 {
		return nil
	}
	if s.mediaBinder == nil {
		return errors.New("media binder is not configured")
	}
	return s.mediaBinder.BindMessage(ctx, message.Id, message.SenderID, message.ReceiverID, message.Attachments)
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

	mongoStart := time.Now()
	message, err := s.service.UpdateViewedStatus(ctx, event.MessageID, event.ActorUserID, status)
	s.observer.ObserveChatMongoOperation("update_viewed_status", resultLabel(err), time.Since(mongoStart))
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
	start := time.Now()
	err := s.rdb.XAck(ctx, s.streamName, consumerGroupName, messageID).Err()
	result := resultLabel(err)
	s.observer.ObserveChatRedisOperation("xack", result, time.Since(start))
	s.observer.ObserveChatStreamAck(result)
	return err
}

func (s *Server) consumerName() string {
	return config.ConsumerName("chat-consumer")
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
		Content:    notificationPreviewForMessage(message),
		Timestamp:  message.Timestamp,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(intent)
	if err != nil {
		return err
	}

	start := time.Now()
	_, err = s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: s.notificationStream,
		Values: map[string]any{"payload": string(payload)},
	}).Result()
	s.observer.ObserveChatRedisOperation("xadd_notification", resultLabel(err), time.Since(start))
	return err
}

func (s *Server) observeStreamState(ctx context.Context) {
	if s.observer == nil {
		return
	}

	start := time.Now()
	groups, err := s.rdb.XInfoGroups(ctx, s.streamName).Result()
	s.observer.ObserveChatRedisOperation("xinfo_groups", resultLabel(err), time.Since(start))
	if err != nil {
		return
	}

	for _, group := range groups {
		if group.Name != consumerGroupName {
			continue
		}
		if group.Lag >= 0 {
			s.observer.SetChatStreamLag(s.streamName, group.Name, float64(group.Lag))
		}
		s.observer.SetChatStreamPending(s.streamName, group.Name, float64(group.Pending))
	}
}

func resultLabel(err error) string {
	if err != nil {
		return "failure"
	}
	return "success"
}

func notificationPreviewForMessage(message *MessageResponse) string {
	if message == nil {
		return ""
	}
	if strings.TrimSpace(message.Content) != "" {
		return message.Content
	}
	if len(message.Attachments) == 0 {
		return ""
	}
	switch message.Attachments[0].Kind {
	case "image":
		return "Image"
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	case "document":
		return "Document"
	default:
		return "File"
	}
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
