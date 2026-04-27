package auth

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) (*MongoRepository, error) {
	repo := &MongoRepository{
		collection: db.Collection("users"),
	}
	if err := repo.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *MongoRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "friend_code", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoRepository) Create(ctx context.Context, registerUserRequest *RegisterRequest) (*User, error) {
	userMongo := UserMongo{
		Username:   registerUserRequest.Username,
		Password:   registerUserRequest.Password,
		Role:       registerUserRequest.Role,
		FriendCode: registerUserRequest.FriendCode,
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
		ID:         userMongo.ID.Hex(),
		Username:   userMongo.Username,
		Password:   userMongo.Password,
		Role:       userMongo.Role,
		FriendCode: userMongo.FriendCode,
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
		ID:         userMongo.ID.Hex(),
		Username:   userMongo.Username,
		Password:   userMongo.Password,
		Role:       userMongo.Role,
		FriendCode: userMongo.FriendCode,
	}

	return user, nil
}

func (r *MongoRepository) FindByFriendCode(ctx context.Context, friendCode string) (*User, error) {
	var userMongo UserMongo
	err := r.collection.FindOne(ctx, bson.M{"friend_code": friendCode}).Decode(&userMongo)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &User{
		ID:         userMongo.ID.Hex(),
		Username:   userMongo.Username,
		Password:   userMongo.Password,
		Role:       userMongo.Role,
		FriendCode: userMongo.FriendCode,
	}, nil
}

func (r *MongoRepository) FindByID(ctx context.Context, userIDHex string) (*User, error) {
	userIDHex = strings.TrimSpace(userIDHex)
	if userIDHex == "" {
		return nil, ErrUserNotFound
	}
	oid, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return nil, ErrUserNotFound
	}

	var userMongo UserMongo
	err = r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&userMongo)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &User{
		ID:         userMongo.ID.Hex(),
		Username:   userMongo.Username,
		Password:   userMongo.Password,
		Role:       userMongo.Role,
		FriendCode: userMongo.FriendCode,
	}, nil
}

func (r *MongoRepository) SetFriendCode(ctx context.Context, userIDHex string, friendCode string) error {
	oid, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		return err
	}

	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"friend_code": friendCode}})
	return err
}
