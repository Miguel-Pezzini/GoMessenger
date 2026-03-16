package friends

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Friend struct {
	ID        string
	OwnerID   string
	Username  string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FriendMongo struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	OwnerID   string             `bson:"owner_id"`
	Username  string             `bson:"username"`
	Name      string             `bson:"name"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}
