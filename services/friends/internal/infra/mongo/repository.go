package mongo

import (
	"context"

	"github.com/Miguel-Pezzini/GoMessenger/services/friends/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	collection *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{collection: db.Collection("friends")}
}

func (r *Repository) Create(ctx context.Context, friend domain.Friend) (domain.Friend, error) {
	doc := domain.FriendMongo{
		OwnerID:   friend.OwnerID,
		Username:  friend.Username,
		Name:      friend.Name,
		CreatedAt: friend.CreatedAt,
		UpdatedAt: friend.UpdatedAt,
	}

	result, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		return domain.Friend{}, err
	}

	oid := result.InsertedID.(primitive.ObjectID)
	friend.ID = oid.Hex()
	return friend, nil
}

func (r *Repository) GetByID(ctx context.Context, ownerID, id string) (domain.Friend, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.Friend{}, mongo.ErrNoDocuments
	}

	var doc domain.FriendMongo
	err = r.collection.FindOne(ctx, bson.M{"_id": oid, "owner_id": ownerID}).Decode(&doc)
	if err != nil {
		return domain.Friend{}, err
	}

	return mapFriendMongo(doc), nil
}

func (r *Repository) ListByOwner(ctx context.Context, ownerID string) ([]domain.Friend, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make([]domain.Friend, 0)
	for cursor.Next(ctx) {
		var doc domain.FriendMongo
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, mapFriendMongo(doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repository) Update(ctx context.Context, ownerID, id string, friend domain.Friend) (domain.Friend, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.Friend{}, mongo.ErrNoDocuments
	}

	result := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": oid, "owner_id": ownerID},
		bson.M{"$set": bson.M{
			"username":   friend.Username,
			"name":       friend.Name,
			"updated_at": friend.UpdatedAt,
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var doc domain.FriendMongo
	if err := result.Decode(&doc); err != nil {
		return domain.Friend{}, err
	}
	return mapFriendMongo(doc), nil
}

func (r *Repository) Delete(ctx context.Context, ownerID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return mongo.ErrNoDocuments
	}

	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": oid, "owner_id": ownerID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func mapFriendMongo(doc domain.FriendMongo) domain.Friend {
	return domain.Friend{
		ID:        doc.ID.Hex(),
		OwnerID:   doc.OwnerID,
		Username:  doc.Username,
		Name:      doc.Name,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	}
}
