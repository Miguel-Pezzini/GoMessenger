package websocket

import (
	"encoding/json"
	"log"
	"os"
)

type Service struct {
	repo *RedisRepository
}

func NewService(repo *RedisRepository) *Service {
	s := &Service{
		repo: repo,
	}
	return s
}

func (s *Service) SubscribeChatChannel(channelName string, handler func(string)) {
	s.repo.Subscribe(channelName, handler)
}

func (s *Service) PersistMessage(msg ChatMessagePayload) error {
	payload, _ := json.Marshal(msg)

	log.Println("Sending to stream", payload)
	if err := s.repo.AddToStream(os.Getenv("REDIS_STREAM_CHAT"), string(payload)); err != nil {
		log.Println("Failed to add message to stream:", err)
		return err
	}

	return nil
}
