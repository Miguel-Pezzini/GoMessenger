package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func startTestServer() (*httptest.Server, string) {
	startBroadcasts()

	s := httptest.NewServer(http.HandlerFunc(WsHandler))
	u := "ws" + s.URL[len("http"):]
	return s, u
}

func TestRegisterUser(t *testing.T) {
	s, u := startTestServer()
	defer s.Close()

	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatal("dial:", err)
	}
	defer c.Close()

	var senderId = "client1"

	newClientMsg := Message{
		Sender:    senderId,
		Type:      "new_client",
		Timestamp: time.Now(),
	}
	if err := c.WriteJSON(newClientMsg); err != nil {
		t.Fatal("write new_client:", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	_, ok := clients[senderId]
	mu.Unlock()
	if !ok {
		t.Fatalf("cliente não registrado")
	}
}

func TestMessagePersistence(t *testing.T) {
	s, u := startTestServer()
	defer s.Close()

	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatal("dial:", err)
	}
	defer c.Close()

	var senderId = "client1"
	var receiverId = "client2"

	c.WriteJSON(Message{Sender: senderId, Type: "new_client", Timestamp: time.Now()})

	msg := Message{
		Text:      "Hello!",
		Sender:    senderId,
		Receiver:  receiverId,
		Type:      "chat",
		Timestamp: time.Now(),
	}
	if err := c.WriteJSON(msg); err != nil {
		t.Fatal("write message:", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	msgs, ok := messages[senderId]
	mu.Unlock()
	if !ok || len(msgs) != 1 || msgs[0].Text != "Hello!" {
		t.Fatalf("mensagem não armazenada corretamente: %+v", msgs)
	}
}

func TestMessageExchange(t *testing.T) {
	s, u := startTestServer()
	defer s.Close()

	c1, _, _ := websocket.DefaultDialer.Dial(u, nil)
	defer c1.Close()
	c2, _, _ := websocket.DefaultDialer.Dial(u, nil)
	defer c2.Close()

	var senderId = "client1"
	var receiverId = "client2"

	c1.WriteJSON(Message{Sender: senderId, Type: "new_client", Timestamp: time.Now()})
	c2.WriteJSON(Message{Sender: receiverId, Type: "new_client", Timestamp: time.Now()})

	msg := Message{
		Text:      "Oi client2!",
		Sender:    senderId,
		Receiver:  receiverId,
		Type:      "chat",
		Timestamp: time.Now(),
	}
	c1.WriteJSON(msg)

	var received Message
	if err := c2.ReadJSON(&received); err != nil {
		t.Fatal("client2 não recebeu mensagem:", err)
	}

	if received.Text != "Oi client2!" || received.Sender != "client1" {
		t.Fatalf("mensagem incorreta recebida: %+v", received)
	}
}

func TestClientDisconnect(t *testing.T) {
	s, u := startTestServer()
	defer s.Close()

	c, _, _ := websocket.DefaultDialer.Dial(u, nil)
	defer c.Close()

	var senderId = "client1"

	c.WriteJSON(Message{Sender: senderId, Type: "new_client", Timestamp: time.Now()})
	time.Sleep(20 * time.Millisecond)

	c.WriteJSON(Message{Sender: senderId, Type: "session_end", Timestamp: time.Now()})
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	conn := clients[senderId]
	mu.Unlock()

	if conn != nil {
		t.Fatalf("cliente não foi removido corretamente")
	}
}
