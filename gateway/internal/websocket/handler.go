package websocket

import (
	"encoding/json"
	"log"
	"net/http"

	"sync"

	"github.com/Miguel-Pezzini/real_time_chat/gateway/internal/auth"
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

	h.clientsM.Lock()
	h.clients[userID] = conn
	h.clientsM.Unlock()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Println("Conexão encerrada:", err)
			break
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Println("Erro ao parsear mensagem:", err)
			continue
		}
		log.Println("Mandando mensagen", msg)
		h.service.SendMessage(msg)
	}
}
