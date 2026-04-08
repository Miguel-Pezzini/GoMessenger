package main

import (
	"log"

	chat "github.com/Miguel-Pezzini/GoMessenger/services/chat/internal"
)

func main() {
	if err := chat.Run(); err != nil {
		log.Fatal(err)
	}
}
