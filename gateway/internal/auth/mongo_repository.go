package auth

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("users"),
	}
}

func (r *MongoRepository) Create(ctx context.Context, user *User) error {
	_, err := r.collection.InsertOne(ctx, user)
	return err
}

func (r *MongoRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
