package main

import (
	"log"

	presence "github.com/Miguel-Pezzini/GoMessenger/services/presence_service/internal"
)

func main() {
	cfg := presence.LoadConfig()
	server, err := presence.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("presence service listening on %s", cfg.Address)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
