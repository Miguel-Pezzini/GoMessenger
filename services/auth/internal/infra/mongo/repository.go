package mongo

import (
	"context"

	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"github.com/Miguel-Pezzini/GoMessenger/services/auth/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repository struct {
	collection *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		collection: db.Collection("users"),
	}
}

func (r *Repository) Create(ctx context.Context, registerUserRequest *authpb.RegisterRequest) (*domain.User, error) {
	userMongo := domain.UserMongo{
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

	user := &domain.User{
		ID:       userMongo.ID.Hex(),
		Username: userMongo.Username,
		Password: userMongo.Password,
	}

	return user, nil
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var userMongo domain.UserMongo
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&userMongo)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:       userMongo.ID.Hex(),
		Username: userMongo.Username,
		Password: userMongo.Password,
	}

	return user, nil
}
