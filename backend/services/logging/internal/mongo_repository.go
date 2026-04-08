package logging

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	gomongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	collection *gomongo.Collection
}

func NewMongoRepository(db *gomongo.Database) (*MongoRepository, error) {
	collection := db.Collection("audit_events")
	indexes := []gomongo.IndexModel{
		{
			Keys:    bson.D{{Key: "stream_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "occurred_at", Value: -1}}},
		{Keys: bson.D{{Key: "service", Value: 1}}},
		{Keys: bson.D{{Key: "category", Value: 1}}},
		{Keys: bson.D{{Key: "event_type", Value: 1}}},
		{Keys: bson.D{{Key: "actor_user_id", Value: 1}}},
	}
	if _, err := collection.Indexes().CreateMany(context.Background(), indexes); err != nil {
		return nil, err
	}
	return &MongoRepository{collection: collection}, nil
}

func (r *MongoRepository) Append(ctx context.Context, event StoredEvent) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"stream_id": event.StreamID},
		bson.M{"$setOnInsert": event},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *MongoRepository) ListRecent(ctx context.Context, limit int) ([]StoredEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "occurred_at", Value: -1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []StoredEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}
