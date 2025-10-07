package websocket

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var (
	clients           = make(map[string]*websocket.Conn) // guarda conexões ativas
	messages          = make(map[string][]Message)       // guarda mensagens por usuário
	mu                sync.Mutex                         // proteção concorrente
	chat_broadcast    = make(chan Message)
	persist_broadcast = make(chan Message)
)

func WsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println(err)
			return
		}
		switch msg.Type {
		case "new_client":
			addNewUser(msg, conn)

		case "chat":
			chat_broadcast <- msg
			persist_broadcast <- msg
		case "session_end":
			removeClient(msg.Sender)
		default:
			log.Println("Tipo de mensagem desconhecido:", msg.Type)
		}
	}
}

func addNewUser(msg Message, conn *websocket.Conn) {
	mu.Lock()
	defer mu.Unlock()
	clients[msg.Sender] = conn
	log.Printf("Novo cliente conectado: %s", msg.Sender)
}

func removeClient(senderId string) {
	mu.Lock()
	defer mu.Unlock()
	clients[senderId] = nil
	log.Printf("Cliente desconectado: %s", senderId)
}
