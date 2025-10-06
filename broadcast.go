package main

import (
	"log"

	"github.com/gorilla/websocket"
)

func startBroadcasts() {
	go func() {
		for msg := range chat_broadcast {
			mu.Lock()
			conn, ok := clients[msg.Receiver]
			mu.Unlock()
			if !ok || conn == nil {
				log.Printf("Receiver %s não conectado", msg.Receiver)
				continue
			}
			sendToReceiver(conn, msg)
		}
	}()

	go func() {
		for msg := range persist_broadcast {
			saveMessage(msg)
		}
	}()
}

func sendToReceiver(conn *websocket.Conn, msg Message) {
	if conn == nil {
		log.Printf("Conexão inválida para %s", msg.Receiver)
		return
	}

	if err := conn.WriteJSON(msg); err != nil {
		log.Println("Erro ao enviar para receiver:", err)
	}
}

func saveMessage(msg Message) {
	mu.Lock()
	defer mu.Unlock()
	messages[msg.Sender] = append(messages[msg.Sender], msg)
	log.Printf("Mensagem persistida para %s: %s", msg.Sender, msg.Text)
}
