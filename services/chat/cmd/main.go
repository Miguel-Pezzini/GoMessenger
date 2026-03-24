package main

import (
	"log"

	"github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
