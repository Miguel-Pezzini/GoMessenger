package main

import (
	"log"
	mongoutil "real_time_chat/internal/mongo"
	redisutil "real_time_chat/internal/redis"
)

func main() {
	db, err := mongoutil.NewMongoClient("mongodb://localhost:27017", "chatdb")
	if err != nil {
		log.Fatal(err)
	}
	redis := redisutil.NewRedisClient()

	server := NewServer(":8080", db, redis)
	log.Println("Gateway running on port 8080")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
