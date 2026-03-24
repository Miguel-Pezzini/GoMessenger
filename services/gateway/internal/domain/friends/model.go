package friends

type Friend struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	FriendID  string `json:"friendId"`
	CreatedAt string `json:"createdAt"`
}

type FriendRequest struct {
	ID         string `json:"id"`
	SenderID   string `json:"senderId"`
	ReceiverID string `json:"receiverId"`
	CreatedAt  string `json:"createdAt"`
}

type SendFriendRequestRequest struct {
	ReceiverID string `json:"receiverId"`
}
