package friends

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Friend struct {
	ID        string
	UserID    string
	FriendID  string
	CreatedAt time.Time
}

type FriendRequest struct {
	ID         string
	SenderID   string
	ReceiverID string
	CreatedAt  time.Time
}

type FriendMongo struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	FriendID  string             `bson:"friend_id"`
	CreatedAt time.Time          `bson:"created_at"`
}

type FriendRequestMongo struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	SenderID   string             `bson:"sender_id"`
	ReceiverID string             `bson:"receiver_id"`
	CreatedAt  time.Time          `bson:"created_at"`
}
