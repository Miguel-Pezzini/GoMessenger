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
		gatewayID: "gateway-1",
		messageCh: make(chan Message),
	}

	repo.Subscribe("chat.gateway."+s.gatewayID, s.HandleIncoming)
	return s
}

func (s *Service) HandleIncoming(payload string) {
	var msg Message
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		log.Println("Erro ao decodificar mensagem:", err)
		return
	}

	log.Printf("📨 Recebido via Redis: %s -> %s: %s\n", msg.SenderID, msg.ReceiverID, msg.Content)
}

func (s *Service) SendMessage(msg Message) error {
	gatewayID, err := s.repo.GetSession(msg.ReceiverID)
	if err != nil {
		log.Println("Receptor não encontrado:", err)
		return err
	}

	if gatewayID == s.gatewayID {
		log.Println("Usuário está neste gateway - entregar localmente (futuro)")
	} else {
		payload, _ := json.Marshal(msg)
		channel := "chat.gateway." + gatewayID
		if err := s.repo.Publish(channel, string(payload)); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) RegisterUser(userID string) {
	s.repo.SetSession(userID, s.gatewayID)
	log.Printf("🔑 Usuário %s registrado no %s\n", userID, s.gatewayID)
}
