package mongo

import (
	"context"

	"github.com/Miguel-Pezzini/GoMessenger/services/chat/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	collection *mongo.Collection
}

type messageDocument struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	StreamID   string             `bson:"stream_id"`
	SenderID   string             `bson:"sender_id"`
	ReceiverID string             `bson:"receiver_id"`
	Content    string             `bson:"content"`
	Timestamp  int64              `bson:"timestamp,omitempty"`
}

func NewRepository(db *mongo.Database) (*Repository, error) {
	repo := &Repository{
		collection: db.Collection("messages"),
	}

	if err := repo.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *Repository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "stream_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *Repository) Create(ctx context.Context, message *domain.MessageDB) (*domain.MessageDB, bool, error) {
	filter := bson.M{"stream_id": message.StreamID}
	update := bson.M{
		"$setOnInsert": bson.M{
			"stream_id":   message.StreamID,
			"sender_id":   message.SenderID,
			"receiver_id": message.ReceiverID,
			"content":     message.Content,
			"timestamp":   message.Timestamp,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return nil, false, err
	}

	var stored messageDocument
	if err := r.collection.FindOne(ctx, filter).Decode(&stored); err != nil {
		return nil, false, err
	}

	return mapMessageDocument(stored), result.UpsertedID != nil, nil
}

func mapMessageDocument(doc messageDocument) *domain.MessageDB {
	return &domain.MessageDB{
		Id:         doc.ID.Hex(),
		StreamID:   doc.StreamID,
		SenderID:   doc.SenderID,
		ReceiverID: doc.ReceiverID,
		Content:    doc.Content,
		Timestamp:  doc.Timestamp,
	}
}
