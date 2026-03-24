package mongo

import (
	"context"
	"fmt"

	"github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repository struct {
	collection *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		collection: db.Collection("messages"),
	}
}

func (r *Repository) Create(ctx context.Context, message *domain.MessageDB) (*domain.MessageDB, error) {
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
