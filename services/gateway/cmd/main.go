package main

import "log"

func main() {
	server := NewServer(":8080", "localhost:50051", "localhost:50052", "http://localhost:8081")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
