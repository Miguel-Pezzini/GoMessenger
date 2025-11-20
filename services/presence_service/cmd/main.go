package main

import "log"

func main() {
	server := NewServer(":8082")
	log.Println("Presence Service running on port 8082")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
