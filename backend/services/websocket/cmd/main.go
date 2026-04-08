package main

import (
	"log"

	websocket "github.com/Miguel-Pezzini/GoMessenger/services/websocket/internal"
)

func main() {
	if err := websocket.Run(); err != nil {
		log.Fatal(err)
	}
}
