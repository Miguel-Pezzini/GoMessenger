package auth

import (
	"context"

	authpb "github.com/Miguel-Pezzini/real_time_chat/auth_service/internal/pb/auth"
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

func (r *MongoRepository) Create(ctx context.Context, registerUserRequest *authpb.RegisterRequest) (*User, error) {
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
		ID:       userMongo.ID.Hex(),
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
