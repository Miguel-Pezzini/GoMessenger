package main

import (
	"log"

	friends "github.com/Miguel-Pezzini/GoMessenger/services/friends/internal"
)

func main() {
	if err := friends.Run(); err != nil {
		log.Fatal(err)
	}
}
