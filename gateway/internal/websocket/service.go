package websocket

import (
	"encoding/json"
	"log"
)

type Service struct {
	repo      *RedisRepository
	gatewayID string
	messageCh chan Message
}

func NewService(repo *RedisRepository) *Service {
	s := &Service{
		repo:      repo,
		messageCh: make(chan Message),
	}
	return s
}

func (s *Service) HandleIncoming(payload string) {
	var msg Message
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		log.Println("Error to unmarshal message", err)
		return
	}
}

func (s *Service) SendMessage(msg Message) error {
	payload, _ := json.Marshal(msg)

	if err := s.repo.AddToStream("message.created", string(payload)); err != nil {
		log.Println("Failed to add message to stream:", err)
		return err
	}

	return nil
}
