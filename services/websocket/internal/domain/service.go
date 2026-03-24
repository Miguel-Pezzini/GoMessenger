package domain

import (
	"encoding/json"
	"log"
)

type Service struct {
	repo       Repository
	streamName string
}

type Repository interface {
	AddToStream(streamName, payload string) error
	Subscribe(channelName string, handler func(payload string))
}

func NewService(repo Repository, streamName string) *Service {
	return &Service{repo: repo, streamName: streamName}
}

func (s *Service) SubscribeChatChannel(channelName string, handler func(string)) {
	s.repo.Subscribe(channelName, handler)
}

func (s *Service) PersistMessage(msg ChatMessagePayload) error {
	payload, _ := json.Marshal(msg)

	log.Println("Sending to stream", payload)
	if err := s.repo.AddToStream(s.streamName, string(payload)); err != nil {
		log.Println("Failed to add message to stream:", err)
		return err
	}

	return nil
}
