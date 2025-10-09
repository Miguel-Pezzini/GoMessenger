package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"real_time_chat/internal/auth"
	"sync"

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

	userID, err := auth.GetUserIDFromRequest(r)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error with authentication"))
		conn.Close()
		return
	}
	if userID == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("user query param required"))
		conn.Close()
		return
	}

	h.clientsM.Lock()
	h.clients[userID] = conn
	h.clientsM.Unlock()

	h.service.RegisterUser(userID)
	log.Printf("👤 %s conectado\n", userID)

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

		h.service.SendMessage(msg)
	}
}
