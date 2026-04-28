package websocket

import (
	"encoding/json"
	"log"
	"strings"
	"time"
)

const maxAttachmentsPerMessage = 10

type Service struct {
	repo        Repository
	streamName  string
	mediaClient MessagePreparer
}

type Repository interface {
	AddToStream(streamName, payload string) error
	Publish(channelName, payload string) error
	Subscribe(channelName string, handler func(payload string))
}

type MessagePreparer interface {
	PrepareMessage(senderID, receiverID string, attachmentIDs []string) ([]AttachmentSnapshot, error)
}

func NewService(repo Repository, streamName string, mediaClients ...MessagePreparer) *Service {
	service := &Service{repo: repo, streamName: streamName}
	if len(mediaClients) > 0 {
		service.mediaClient = mediaClients[0]
	}
	return service
}

func (s *Service) SubscribeChatChannel(channelName string, handler func(string)) {
	s.repo.Subscribe(channelName, handler)
}

func (s *Service) PublishPresenceConnected(channelName, userID string) error {
	return s.publishPresenceEvent(channelName, PresenceLifecycleEvent{
		UserID:     userID,
		Type:       "connected",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Service) PublishPresenceDisconnected(channelName, userID string) error {
	return s.publishPresenceEvent(channelName, PresenceLifecycleEvent{
		UserID:     userID,
		Type:       "disconnected",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Service) PublishPresenceChatOpened(channelName, userID, currentChatID string) error {
	if currentChatID == "" {
		return ValidationError{Message: "current_chat_id is required"}
	}

	return s.publishPresenceEvent(channelName, PresenceLifecycleEvent{
		UserID:        userID,
		Type:          MessageTypeChatOpened,
		CurrentChatID: currentChatID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Service) PublishPresenceChatClosed(channelName, userID string) error {
	return s.publishPresenceEvent(channelName, PresenceLifecycleEvent{
		UserID:     userID,
		Type:       MessageTypeChatClosed,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Service) PublishChatInteraction(channelName, actorUserID, eventType string, payload ChatInteractionPayload) error {
	event := ChatInteractionEvent{
		Type:          eventType,
		ActorUserID:   actorUserID,
		TargetUserID:  payload.TargetUserID,
		CurrentChatID: payload.CurrentChatID,
		MessageID:     payload.MessageID,
		ViewedStatus:  viewedStatusForEvent(eventType),
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	}

	if err := validateChatInteractionEvent(event); err != nil {
		return err
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return s.repo.Publish(channelName, string(payloadBytes))
}

func (s *Service) publishPresenceEvent(channelName string, event PresenceLifecycleEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return s.repo.Publish(channelName, string(payload))
}

func (s *Service) PersistMessage(authenticatedUserID string, msg ChatMessagePayload) error {
	if msg.SenderID != "" && msg.SenderID != authenticatedUserID {
		return ValidationError{Message: "sender_id does not match authenticated user"}
	}
	if strings.TrimSpace(msg.ReceiverID) == "" {
		return ValidationError{Message: "receiver_id is required"}
	}

	attachmentIDs, err := normalizeAttachmentIDs(msg.AttachmentIDs)
	if err != nil {
		return err
	}
	if strings.TrimSpace(msg.Content) == "" && len(attachmentIDs) == 0 {
		return ValidationError{Message: "content or attachment_ids is required"}
	}

	msg.SenderID = authenticatedUserID
	msg.ReceiverID = strings.TrimSpace(msg.ReceiverID)
	msg.Content = strings.TrimSpace(msg.Content)

	var attachments []AttachmentSnapshot
	if len(attachmentIDs) > 0 {
		if s.mediaClient == nil {
			return ValidationError{Message: "media service is not configured"}
		}
		attachments, err = s.mediaClient.PrepareMessage(msg.SenderID, msg.ReceiverID, attachmentIDs)
		if err != nil {
			return err
		}
	}

	payload, err := json.Marshal(ChatStreamPayload{
		SenderID:    msg.SenderID,
		ReceiverID:  msg.ReceiverID,
		Content:     msg.Content,
		Attachments: attachments,
	})
	if err != nil {
		return err
	}

	log.Println("Sending to stream", payload)
	if err := s.repo.AddToStream(s.streamName, string(payload)); err != nil {
		log.Println("Failed to add message to stream:", err)
		return err
	}

	return nil
}

func normalizeAttachmentIDs(ids []string) ([]string, error) {
	if len(ids) > maxAttachmentsPerMessage {
		return nil, ValidationError{Message: "too many attachments"}
	}

	seen := map[string]bool{}
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, ValidationError{Message: "attachment_id is required"}
		}
		if seen[id] {
			return nil, ValidationError{Message: "duplicate attachment_id"}
		}
		seen[id] = true
		clean = append(clean, id)
	}
	return clean, nil
}

func viewedStatusForEvent(eventType string) string {
	switch eventType {
	case MessageTypeMessageDelivered:
		return ViewedStatusDelivered
	case MessageTypeMessageSeen:
		return ViewedStatusSeen
	default:
		return ""
	}
}

func validateChatInteractionEvent(event ChatInteractionEvent) error {
	if event.ActorUserID == "" {
		return ValidationError{Message: "actor_user_id is required"}
	}

	switch event.Type {
	case MessageTypeTypingStarted, MessageTypeTypingStopped:
		if event.TargetUserID == "" {
			return ValidationError{Message: "target_user_id is required"}
		}
	case MessageTypeMessageDelivered, MessageTypeMessageSeen:
		if event.TargetUserID == "" {
			return ValidationError{Message: "target_user_id is required"}
		}
		if event.MessageID == "" {
			return ValidationError{Message: "message_id is required"}
		}
	default:
		return ValidationError{Message: "unsupported interaction event type"}
	}

	return nil
}
