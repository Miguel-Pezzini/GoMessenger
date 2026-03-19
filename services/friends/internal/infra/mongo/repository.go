package mongo

import (
	"context"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/services/friends/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	client             *mongo.Client
	friendsCollection  *mongo.Collection
	requestsCollection *mongo.Collection
}

func NewRepository(db *mongo.Database) (*Repository, error) {
	repo := &Repository{
		client:             db.Client(),
		friendsCollection:  db.Collection("friends"),
		requestsCollection: db.Collection("friend_requests"),
	}

	if err := repo.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *Repository) ensureIndexes(ctx context.Context) error {
	friendIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "friend_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
		},
	}

	requestIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "sender_id", Value: 1}, {Key: "receiver_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "receiver_id", Value: 1}, {Key: "created_at", Value: -1}},
		},
	}

	if _, err := r.friendsCollection.Indexes().CreateMany(ctx, friendIndexes); err != nil {
		return err
	}
	if _, err := r.requestsCollection.Indexes().CreateMany(ctx, requestIndexes); err != nil {
		return err
	}
	return nil
}

func (r *Repository) CreateFriendRequest(ctx context.Context, request domain.FriendRequest) (domain.FriendRequest, error) {
	doc := domain.FriendRequestMongo{
		SenderID:   request.SenderID,
		ReceiverID: request.ReceiverID,
		CreatedAt:  request.CreatedAt,
	}

	result, err := r.requestsCollection.InsertOne(ctx, doc)
	if err != nil {
		return domain.FriendRequest{}, err
	}

	request.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return request, nil
}

func (r *Repository) GetFriendRequestByID(ctx context.Context, requestID string) (domain.FriendRequest, error) {
	oid, err := primitive.ObjectIDFromHex(requestID)
	if err != nil {
		return domain.FriendRequest{}, mongo.ErrNoDocuments
	}

	var doc domain.FriendRequestMongo
	err = r.requestsCollection.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if err != nil {
		return domain.FriendRequest{}, err
	}

	return mapFriendRequestMongo(doc), nil
}

func (r *Repository) ListPendingFriendRequests(ctx context.Context, receiverID string) ([]domain.FriendRequest, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.requestsCollection.Find(ctx, bson.M{"receiver_id": receiverID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make([]domain.FriendRequest, 0)
	for cursor.Next(ctx) {
		var doc domain.FriendRequestMongo
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, mapFriendRequestMongo(doc))
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repository) DeleteFriendRequestByID(ctx context.Context, requestID string) error {
	oid, err := primitive.ObjectIDFromHex(requestID)
	if err != nil {
		return mongo.ErrNoDocuments
	}

	result, err := r.requestsCollection.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *Repository) FriendRequestExistsBetween(ctx context.Context, firstUserID, secondUserID string) (bool, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"sender_id": firstUserID, "receiver_id": secondUserID},
			{"sender_id": secondUserID, "receiver_id": firstUserID},
		},
	}

	count, err := r.requestsCollection.CountDocuments(ctx, filter)
	return count > 0, err
}

func (r *Repository) FriendshipExists(ctx context.Context, userID, friendID string) (bool, error) {
	count, err := r.friendsCollection.CountDocuments(ctx, bson.M{"user_id": userID, "friend_id": friendID})
	return count > 0, err
}

func (r *Repository) CreateFriendships(ctx context.Context, firstUserID, secondUserID string, createdAt time.Time) error {
	_, err := r.friendsCollection.InsertMany(ctx, []any{
		domain.FriendMongo{
			UserID:    firstUserID,
			FriendID:  secondUserID,
			CreatedAt: createdAt,
		},
		domain.FriendMongo{
			UserID:    secondUserID,
			FriendID:  firstUserID,
			CreatedAt: createdAt,
		},
	})
	return err
}

func (r *Repository) DeleteFriendships(ctx context.Context, firstUserID, secondUserID string) error {
	filter := bson.M{
		"$or": []bson.M{
			{"user_id": firstUserID, "friend_id": secondUserID},
			{"user_id": secondUserID, "friend_id": firstUserID},
		},
	}

	result, err := r.friendsCollection.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *Repository) ListFriends(ctx context.Context, userID string) ([]domain.Friend, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.friendsCollection.Find(ctx, bson.M{"user_id": userID}, opts)
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

func (r *Repository) RunInTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	session, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, fn(sessCtx)
	})
	return err
}

func mapFriendMongo(doc domain.FriendMongo) domain.Friend {
	return domain.Friend{
		ID:        doc.ID.Hex(),
		UserID:    doc.UserID,
		FriendID:  doc.FriendID,
		CreatedAt: doc.CreatedAt,
	}
}

func mapFriendRequestMongo(doc domain.FriendRequestMongo) domain.FriendRequest {
	return domain.FriendRequest{
		ID:         doc.ID.Hex(),
		SenderID:   doc.SenderID,
		ReceiverID: doc.ReceiverID,
		CreatedAt:  doc.CreatedAt,
	}
}
