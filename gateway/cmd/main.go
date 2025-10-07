package main

import (
	"log"
	"real_time_chat/internal"
)

func main() {
	server := internal.NewServer(":8080")
	log.Println("Gateway running on port 8080")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
