package auth

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserMongo struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username string             `bson:"name" json:"username"`
	Password string             `bson:"email" json:"password"`
}

type RegisterUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}
