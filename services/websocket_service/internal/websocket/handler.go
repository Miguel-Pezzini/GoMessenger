package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WsHandler struct {
	service  *Service
	clients  map[string]*websocket.Conn
	clientsM sync.RWMutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func NewWsHandler(service *Service) *WsHandler {
	return &WsHandler{
		service: service,
		clients: make(map[string]*websocket.Conn),
	}
}

func (h *WsHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
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

		var gatewayMessage GatewayMessage
		if err := json.Unmarshal(msgBytes, &gatewayMessage); err != nil {
			log.Println("Erro ao parsear mensagem:", err)
			continue
		}
		switch gatewayMessage.Type {
		case MessageTypeChat:
			var payload ChatMessagePayload
			if err := json.Unmarshal(gatewayMessage.Payload, &payload); err != nil {
				log.Println("Erro ao parsear payload de chat:", err)
				continue
			}
			h.service.PersistMessage(payload)
		}
	}

	h.clientsM.Lock()
	delete(h.clients, userID)
	h.clientsM.Unlock()
}

func (h *WsHandler) StartPubSubListener() {
	go h.service.SubscribeChatChannel(os.Getenv("REDIS_CHANNEL_CHAT"), func(payload string) {
		var msg MessageResponse
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

func (h *WsHandler) startPingLoop(conn *websocket.Conn) {
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
