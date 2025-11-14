package chat

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("messages"),
	}
}

func (r *MongoRepository) Create(ctx context.Context, message *MessageDB) (*MessageDB, error) {
	result, err := r.collection.InsertOne(ctx, message)
	if err != nil {
		return nil, err
	}

	oid, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return nil, fmt.Errorf("failed to convert inserted ID")
	}
	message.Id = oid.Hex()
	return message, nil
}
