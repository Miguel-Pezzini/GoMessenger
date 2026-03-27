package http

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/services/websocket/internal/domain"
	"github.com/gorilla/websocket"
)

type Handler struct {
	service  *domain.Service
	clients  map[string]*websocket.Conn
	clientsM sync.RWMutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func NewHandler(service *domain.Service) *Handler {
	return &Handler{
		service: service,
		clients: make(map[string]*websocket.Conn),
	}
}

func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "missing user id", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Erro ao fazer upgrade:", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	h.clientsM.Lock()
	h.clients[userID] = conn
	h.clientsM.Unlock()

	go h.startPingLoop(conn)

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Println("Conexão encerrada:", err)
			break
		}

		var gatewayMessage domain.GatewayMessage
		if err := json.Unmarshal(msgBytes, &gatewayMessage); err != nil {
			log.Println("Erro ao parsear mensagem:", err)
			continue
		}
		switch gatewayMessage.Type {
		case domain.MessageTypeChat:
			var payload domain.ChatMessagePayload
			if err := json.Unmarshal(gatewayMessage.Payload, &payload); err != nil {
				log.Println("Erro ao parsear payload de chat:", err)
				continue
			}
			if err := h.service.PersistMessage(userID, payload); err != nil {
				log.Println("Failed to persist message:", err)
				if validationErr, ok := err.(domain.ValidationError); ok {
					_ = conn.WriteJSON(domain.ErrorResponse{
						Type:  domain.MessageTypeError,
						Error: validationErr,
					})
				}
				continue
			}
		}
	}

	h.clientsM.Lock()
	delete(h.clients, userID)
	h.clientsM.Unlock()
}

func (h *Handler) StartPubSubListener(channelName string) {
	go h.service.SubscribeChatChannel(channelName, func(payload string) {
		var msg domain.MessageResponse
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			log.Println("Erro ao parsear mensagem Pub/Sub:", err)
			return
		}

		h.clientsM.Lock()
		defer h.clientsM.Unlock()

		if conn, ok := h.clients[msg.ReceiverID]; ok {
			_ = conn.WriteJSON(msg)
		}
		if conn, ok := h.clients[msg.SenderID]; ok {
			_ = conn.WriteJSON(msg)
		}
	})
}

func (h *Handler) startPingLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
			log.Println("Ping error, closing connection:", err)
			_ = conn.Close()
			return
		}
	}
}
