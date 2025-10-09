package mongoutil

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewMongoClient(URI, dbName string) (*mongo.Database, error) {
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
