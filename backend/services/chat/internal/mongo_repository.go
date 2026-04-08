package chat

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

type messageDocument struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	StreamID     string             `bson:"stream_id"`
	SenderID     string             `bson:"sender_id"`
	ReceiverID   string             `bson:"receiver_id"`
	Content      string             `bson:"content"`
	Timestamp    int64              `bson:"timestamp,omitempty"`
	ViewedStatus string             `bson:"viewed_status,omitempty"`
}

func NewMongoRepository(db *mongo.Database) (*MongoRepository, error) {
	repo := &MongoRepository{
		collection: db.Collection("messages"),
	}

	if err := repo.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *MongoRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "stream_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		// speeds up conversation queries: (sender_id, receiver_id, _id)
		{
			Keys: bson.D{
				{Key: "sender_id", Value: 1},
				{Key: "receiver_id", Value: 1},
				{Key: "_id", Value: -1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoRepository) Create(ctx context.Context, message *MessageDB) (*MessageDB, bool, error) {
	filter := bson.M{"stream_id": message.StreamID}
	update := bson.M{
		"$setOnInsert": bson.M{
			"stream_id":     message.StreamID,
			"sender_id":     message.SenderID,
			"receiver_id":   message.ReceiverID,
			"content":       message.Content,
			"timestamp":     message.Timestamp,
			"viewed_status": NormalizeViewedStatus(message.ViewedStatus),
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

func (r *MongoRepository) UpdateViewedStatus(ctx context.Context, messageID, receiverUserID, status string) (*MessageDB, error) {
	objectID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"_id":         objectID,
		"receiver_id": receiverUserID,
	}

	var stored messageDocument
	if err := r.collection.FindOne(ctx, filter).Decode(&stored); err != nil {
		return nil, err
	}

	currentStatus := NormalizeViewedStatus(stored.ViewedStatus)
	targetStatus := NormalizeViewedStatus(status)
	if ViewedStatusRank(currentStatus) >= ViewedStatusRank(targetStatus) {
		stored.ViewedStatus = currentStatus
		return mapMessageDocument(stored), nil
	}

	if _, err := r.collection.UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{"viewed_status": targetStatus},
	}); err != nil {
		return nil, err
	}

	stored.ViewedStatus = targetStatus
	return mapMessageDocument(stored), nil
}

func (r *MongoRepository) GetConversation(ctx context.Context, userA, userB, before string, limit int) ([]MessageDB, error) {
	filter := bson.M{
		"$or": bson.A{
			bson.M{"sender_id": userA, "receiver_id": userB},
			bson.M{"sender_id": userB, "receiver_id": userA},
		},
	}

	if before != "" {
		beforeID, err := primitive.ObjectIDFromHex(before)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$lt": beforeID}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []messageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	messages := make([]MessageDB, len(docs))
	for i, doc := range docs {
		messages[i] = *mapMessageDocument(doc)
	}
	return messages, nil
}

func mapMessageDocument(doc messageDocument) *MessageDB {
	return &MessageDB{
		Id:           doc.ID.Hex(),
		StreamID:     doc.StreamID,
		SenderID:     doc.SenderID,
		ReceiverID:   doc.ReceiverID,
		Content:      doc.Content,
		Timestamp:    doc.Timestamp,
		ViewedStatus: NormalizeViewedStatus(doc.ViewedStatus),
	}
}
