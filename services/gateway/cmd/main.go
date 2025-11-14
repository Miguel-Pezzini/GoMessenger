package main

import (
	"log"
)

func main() {
	server := NewServer(":8080", "localhost:50051")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
