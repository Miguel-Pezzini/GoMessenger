package media

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	collection *mongo.Collection
}

type attachmentDocument struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	OwnerID           string             `bson:"owner_id"`
	ObjectKey         string             `bson:"object_key"`
	Filename          string             `bson:"filename"`
	ContentType       string             `bson:"content_type"`
	Size              int64              `bson:"size"`
	Kind              string             `bson:"kind"`
	Status            string             `bson:"status"`
	BoundMessageID    string             `bson:"bound_message_id,omitempty"`
	AuthorizedUserIDs []string           `bson:"authorized_user_ids"`
	CreatedAt         time.Time          `bson:"created_at"`
	BoundAt           time.Time          `bson:"bound_at,omitempty"`
	ExpiresAt         *time.Time         `bson:"expires_at,omitempty"`
}

func NewMongoRepository(db *mongo.Database) (*MongoRepository, error) {
	repo := &MongoRepository{collection: db.Collection("attachments")}
	if err := repo.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *MongoRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "status", Value: 1}, {Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "authorized_user_ids", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "expires_at", Value: 1}},
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoRepository) Create(ctx context.Context, attachment *Attachment) error {
	doc, err := attachmentToDocument(*attachment)
	if err != nil {
		return err
	}
	_, err = r.collection.InsertOne(ctx, doc)
	return err
}

func (r *MongoRepository) FindByID(ctx context.Context, id string) (*Attachment, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, ErrNotFound
	}

	var doc attachmentDocument
	if err := r.collection.FindOne(ctx, bson.M{"_id": objectID, "status": bson.M{"$ne": StatusDeleted}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return documentToAttachment(doc), nil
}

func (r *MongoRepository) FindByIDs(ctx context.Context, ids []string) ([]Attachment, error) {
	objectIDs := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		objectID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, ErrNotFound
		}
		objectIDs = append(objectIDs, objectID)
	}

	cursor, err := r.collection.Find(ctx, bson.M{
		"_id":    bson.M{"$in": objectIDs},
		"status": bson.M{"$ne": StatusDeleted},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []attachmentDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	attachments := make([]Attachment, len(docs))
	for i, doc := range docs {
		attachments[i] = *documentToAttachment(doc)
	}
	return attachments, nil
}

func (r *MongoRepository) Bind(ctx context.Context, ids []string, messageID, senderID, receiverID string) ([]Attachment, error) {
	attachments, err := r.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(attachments) != len(ids) {
		return nil, ErrNotFound
	}

	now := time.Now().UTC()
	byID := make(map[string]Attachment, len(attachments))
	for _, attachment := range attachments {
		if attachment.OwnerID != senderID {
			return nil, ErrForbidden
		}
		if attachment.Status == StatusBound && attachment.BoundMessageID != messageID {
			return nil, ErrForbidden
		}
		if attachment.Status != StatusUploaded && attachment.Status != StatusBound {
			return nil, ErrForbidden
		}

		if attachment.Status == StatusUploaded {
			objectID, err := primitive.ObjectIDFromHex(attachment.ID)
			if err != nil {
				return nil, ErrNotFound
			}
			_, err = r.collection.UpdateOne(ctx, bson.M{
				"_id":      objectID,
				"owner_id": senderID,
				"status":   StatusUploaded,
			}, bson.M{
				"$set": bson.M{
					"status":              StatusBound,
					"bound_message_id":    messageID,
					"authorized_user_ids": uniqueUsers(senderID, receiverID),
					"bound_at":            now,
				},
				"$unset": bson.M{"expires_at": ""},
			})
			if err != nil {
				return nil, err
			}
			attachment.Status = StatusBound
			attachment.BoundMessageID = messageID
			attachment.AuthorizedUserIDs = uniqueUsers(senderID, receiverID)
			attachment.BoundAt = now
			attachment.ExpiresAt = nil
		}
		byID[attachment.ID] = attachment
	}

	ordered := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		ordered = append(ordered, byID[id])
	}
	return ordered, nil
}

func (r *MongoRepository) ListExpired(ctx context.Context, now time.Time, limit int) ([]Attachment, error) {
	if limit <= 0 {
		limit = 100
	}

	cursor, err := r.collection.Find(ctx, bson.M{
		"status":     StatusUploaded,
		"expires_at": bson.M{"$lte": now},
	}, options.Find().SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []attachmentDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	attachments := make([]Attachment, len(docs))
	for i, doc := range docs {
		attachments[i] = *documentToAttachment(doc)
	}
	return attachments, nil
}

func (r *MongoRepository) MarkDeleted(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return ErrNotFound
	}
	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{
		"$set": bson.M{"status": StatusDeleted},
	})
	return err
}

func attachmentToDocument(attachment Attachment) (attachmentDocument, error) {
	objectID, err := primitive.ObjectIDFromHex(attachment.ID)
	if err != nil {
		return attachmentDocument{}, err
	}
	return attachmentDocument{
		ID:                objectID,
		OwnerID:           attachment.OwnerID,
		ObjectKey:         attachment.ObjectKey,
		Filename:          attachment.Filename,
		ContentType:       attachment.ContentType,
		Size:              attachment.Size,
		Kind:              attachment.Kind,
		Status:            attachment.Status,
		BoundMessageID:    attachment.BoundMessageID,
		AuthorizedUserIDs: attachment.AuthorizedUserIDs,
		CreatedAt:         attachment.CreatedAt,
		BoundAt:           attachment.BoundAt,
		ExpiresAt:         attachment.ExpiresAt,
	}, nil
}

func documentToAttachment(doc attachmentDocument) *Attachment {
	return &Attachment{
		ID:                doc.ID.Hex(),
		OwnerID:           doc.OwnerID,
		ObjectKey:         doc.ObjectKey,
		Filename:          doc.Filename,
		ContentType:       doc.ContentType,
		Size:              doc.Size,
		Kind:              doc.Kind,
		Status:            doc.Status,
		BoundMessageID:    doc.BoundMessageID,
		AuthorizedUserIDs: doc.AuthorizedUserIDs,
		CreatedAt:         doc.CreatedAt,
		BoundAt:           doc.BoundAt,
		ExpiresAt:         doc.ExpiresAt,
	}
}

func uniqueUsers(users ...string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(users))
	for _, user := range users {
		if user == "" || seen[user] {
			continue
		}
		seen[user] = true
		result = append(result, user)
	}
	return result
}
