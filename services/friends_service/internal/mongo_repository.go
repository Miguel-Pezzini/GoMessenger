package friends

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{collection: db.Collection("friends")}
}

func (r *MongoRepository) Create(ctx context.Context, friend Friend) (Friend, error) {
	doc := FriendMongo{
		OwnerID:   friend.OwnerID,
		Username:  friend.Username,
		Name:      friend.Name,
		CreatedAt: friend.CreatedAt,
		UpdatedAt: friend.UpdatedAt,
	}

	result, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		return Friend{}, err
	}

	oid := result.InsertedID.(primitive.ObjectID)
	friend.ID = oid.Hex()
	return friend, nil
}

func (r *MongoRepository) GetByID(ctx context.Context, ownerID, id string) (Friend, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return Friend{}, mongo.ErrNoDocuments
	}

	var doc FriendMongo
	err = r.collection.FindOne(ctx, bson.M{"_id": oid, "owner_id": ownerID}).Decode(&doc)
	if err != nil {
		return Friend{}, err
	}

	return mapFriendMongo(doc), nil
}

func (r *MongoRepository) ListByOwner(ctx context.Context, ownerID string) ([]Friend, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make([]Friend, 0)
	for cursor.Next(ctx) {
		var doc FriendMongo
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

func (r *MongoRepository) Update(ctx context.Context, ownerID, id string, friend Friend) (Friend, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return Friend{}, mongo.ErrNoDocuments
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

	var doc FriendMongo
	if err := result.Decode(&doc); err != nil {
		return Friend{}, err
	}
	return mapFriendMongo(doc), nil
}

func (r *MongoRepository) Delete(ctx context.Context, ownerID, id string) error {
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

func mapFriendMongo(doc FriendMongo) Friend {
	return Friend{
		ID:        doc.ID.Hex(),
		OwnerID:   doc.OwnerID,
		Username:  doc.Username,
		Name:      doc.Name,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	}
}
