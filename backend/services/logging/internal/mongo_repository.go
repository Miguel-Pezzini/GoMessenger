package logging

import (
	"context"
	"regexp"
	"strings"

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
	return r.List(ctx, LogFilter{Limit: limit})
}

func (r *MongoRepository) List(ctx context.Context, filter LogFilter) ([]StoredEvent, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := bson.M{}
	addExact := func(field, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			query[field] = value
		}
	}
	addExact("service", filter.Service)
	addExact("category", filter.Category)
	addExact("status", filter.Status)
	addExact("event_type", filter.EventType)
	addExact("actor_user_id", filter.ActorUserID)

	if q := strings.TrimSpace(filter.Query); q != "" {
		pattern := regexp.QuoteMeta(q)
		regex := bson.M{"$regex": pattern, "$options": "i"}
		query["$or"] = bson.A{
			bson.M{"event_type": regex},
			bson.M{"category": regex},
			bson.M{"service": regex},
			bson.M{"status": regex},
			bson.M{"message": regex},
			bson.M{"actor_user_id": regex},
			bson.M{"target_user_id": regex},
			bson.M{"entity_type": regex},
			bson.M{"entity_id": regex},
			bson.M{"metadata.username": regex},
			bson.M{"metadata.error": regex},
		}
	}

	cursor, err := r.collection.Find(ctx, query, options.Find().
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
