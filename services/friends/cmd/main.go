package main

import (
	"log"

	"github.com/Miguel-Pezzini/GoMessenger/services/friends/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
