package auth

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("users"),
	}
}

func (r *MongoRepository) Create(ctx context.Context, registerUserRequest *RegisterRequest) (*User, error) {
	userMongo := UserMongo{
		Username: registerUserRequest.Username,
		Password: registerUserRequest.Password,
		Role:     registerUserRequest.Role,
	}

	result, err := r.collection.InsertOne(ctx, userMongo)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrUserAlreadyExists
		}
		return nil, err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		userMongo.ID = oid
	}

	user := &User{
		ID:       userMongo.ID.Hex(),
		Username: userMongo.Username,
		Password: userMongo.Password,
		Role:     userMongo.Role,
	}

	return user, nil
}

func (r *MongoRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	var userMongo UserMongo
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&userMongo)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	user := &User{
		ID:       userMongo.ID.Hex(),
		Username: userMongo.Username,
		Password: userMongo.Password,
		Role:     userMongo.Role,
	}

	return user, nil
}
