package domain

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req MessageRequest) (*MessageResponse, error) {
	messageDB := &MessageDB{
		StreamID:   req.StreamID,
		SenderID:   req.SenderID,
		ReceiverID: req.ReceiverID,
		Content:    req.Content,
		Timestamp:  req.Timestamp,
	}
	result, _, err := s.repo.Create(ctx, messageDB)
	if err != nil {
		return nil, err
	}
	return MessageResponseFromMessageDB(result), nil
}
