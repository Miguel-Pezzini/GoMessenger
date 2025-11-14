package main

import (
	"log"
)

func main() {
	server := NewServer(":8081")
	log.Println("Chat Service running on port 8081")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
