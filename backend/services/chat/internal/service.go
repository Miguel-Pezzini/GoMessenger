package chat

import (
	"context"
	"errors"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req MessageRequest) (*MessageResponse, error) {
	messageDB := &MessageDB{
		StreamID:     req.StreamID,
		SenderID:     req.SenderID,
		ReceiverID:   req.ReceiverID,
		Content:      req.Content,
		Timestamp:    req.Timestamp,
		ViewedStatus: ViewedStatusSent,
	}
	result, _, err := s.repo.Create(ctx, messageDB)
	if err != nil {
		return nil, err
	}
	return MessageResponseFromMessageDB(result), nil
}

const defaultConversationLimit = 20

func (s *Service) GetConversation(ctx context.Context, userA, userB, before string, limit int) (*ConversationResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = defaultConversationLimit
	}

	// fetch one extra to detect if there are more pages
	messages, err := s.repo.GetConversation(ctx, userA, userB, before, limit+1)
	if err != nil {
		return nil, err
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	// reverse so the slice is oldest → newest (repo returns newest-first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	responses := make([]MessageResponse, len(messages))
	for i, m := range messages {
		responses[i] = *MessageResponseFromMessageDB(&m)
	}

	return &ConversationResponse{Messages: responses, HasMore: hasMore}, nil
}

func (s *Service) UpdateViewedStatus(ctx context.Context, messageID, receiverUserID, status string) (*MessageResponse, error) {
	if messageID == "" {
		return nil, errors.New("message_id is required")
	}
	if receiverUserID == "" {
		return nil, errors.New("receiver_user_id is required")
	}

	status = NormalizeViewedStatus(status)
	if status == ViewedStatusSent {
		return nil, errors.New("viewed_status must be delivered or seen")
	}

	message, err := s.repo.UpdateViewedStatus(ctx, messageID, receiverUserID, status)
	if err != nil {
		return nil, err
	}

	return MessageResponseFromMessageDB(message), nil
}
