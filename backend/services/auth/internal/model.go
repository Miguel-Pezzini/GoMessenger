package auth

import "go.mongodb.org/mongo-driver/bson/primitive"

const (
	RoleUser  = "USER"
	RoleAdmin = "ADMIN"
)

type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Role       string `json:"role"`
	FriendCode string `json:"friendCode,omitempty"`
}

type RegisterRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Role       string `json:"role,omitempty"`
	FriendCode string `json:"-"` // set by service before persistence
}

type RegisterResponse struct {
	Token      string `json:"token"`
	Role       string `json:"role"`
	FriendCode string `json:"friendCode"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token      string `json:"token"`
	Role       string `json:"role"`
	FriendCode string `json:"friendCode"`
}

type UserMongo struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username   string             `bson:"username" json:"username"`
	Password   string             `bson:"password" json:"password"`
	Role       string             `bson:"role" json:"role"`
	FriendCode string             `bson:"friend_code,omitempty" json:"friendCode,omitempty"`
}
