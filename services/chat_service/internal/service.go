package chat

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req MessageRequest) (*MessageResponse, error) {
	messageDB := &MessageDB{
		SenderID:   req.SenderID,
		ReceiverID: req.ReceiverID,
		Content:    req.Content,
		Timestamp:  req.Timestamp,
	}
	result, err := s.repo.Create(ctx, messageDB)
	if err != nil {
		return nil, err
	}
	return MessageResponseFromMessageDB(result), nil
}
