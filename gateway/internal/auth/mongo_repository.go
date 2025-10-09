package auth

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

func (r *MongoRepository) Create(ctx context.Context, registerUserRequest *RegisterUserRequest) (*User, error) {
	userMongo := UserMongo{
		Username: registerUserRequest.Username,
		Password: registerUserRequest.Password,
	}

	result, err := r.collection.InsertOne(ctx, userMongo)
	if err != nil {
		return nil, err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		userMongo.ID = oid
	}

	user := &User{
		ID:       int(userMongo.ID.Timestamp().Unix()),
		Username: userMongo.Username,
		Password: userMongo.Password,
	}

	return user, nil
}

func (r *MongoRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
