package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"sync"

	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/auth"
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Erro ao fazer upgrade:", err)
		return
	}
	userID := r.Context().Value(auth.UserIDKey).(string)
	if userID == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("user query param required"))
		conn.Close()
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

	go h.startPingLoop(userID, conn)

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
			{
				var payload ChatMessagePayload
				json.Unmarshal(gatewayMessage.Payload, &payload)
				h.service.PersistMessage(payload)
			}
		}
	}

	h.clientsM.Lock()
	delete(h.clients, userID)
	h.clientsM.Unlock()
}

func (h *WsHandler) StartPubSubListener() {
	h.service.SubscribeChatChannel(os.Getenv("REDIS_CHANNEL_CHAT"), func(payload string) {
		var msg MessageResponse
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			log.Println("Erro ao parsear mensagem Pub/Sub:", err)
			return
		}

		h.clientsM.Lock()
		defer h.clientsM.Unlock()

		if conn, ok := h.clients[msg.ReceiverID]; ok {
			conn.WriteJSON(msg)
		}
		if conn, ok := h.clients[msg.SenderID]; ok {
			conn.WriteJSON(msg)
		}
	})
}

func (h *WsHandler) startPingLoop(userID string, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteControl(
			websocket.PingMessage,
			[]byte{},
			time.Now().Add(5*time.Second),
		); err != nil {
			log.Println("Ping error, closing connection:", err)
			conn.Close()
			return
		}
	}
}
