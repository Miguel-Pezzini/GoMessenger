package main

import (
	"log"

	"github.com/Miguel-Pezzini/real_time_chat/chat_service/internal/mongo"
	db "github.com/Miguel-Pezzini/real_time_chat/pkg/db"
)

func main() {
	mongoDB, err := mongo.NewMongoClient("mongodb://localhost:27018", "chatdb")
	if err != nil {
		log.Fatalf("failed to connecting to chat database: %v", err)
	}
	redisClient, err := db.NewRedisClient()
	if err != nil {
		log.Fatal("error connecting with redis", err)
	}
	log.Println("Gateway connected with Authentication Service")

	server := NewServer(":8080", redisClient)
	log.Println("Gateway running on port 8080")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
