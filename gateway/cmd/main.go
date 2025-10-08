package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	db, err := ConnectMongoDB("mongodb://localhost:27017", "chatdb")
	if err != nil {
		log.Fatal(err)
	}

	server := NewServer(":8080", db)
	log.Println("Gateway running on port 8080")
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}

func ConnectMongoDB(URI, dbName string) (*mongo.Database, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(URI))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	return client.Database(dbName), nil
}
