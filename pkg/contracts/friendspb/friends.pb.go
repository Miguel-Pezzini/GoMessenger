package friendspb

type SendFriendRequestRequest struct {
	SenderId   string `json:"senderId"`
	ReceiverId string `json:"receiverId"`
}

type AcceptFriendRequestRequest struct {
	ActorId   string `json:"actorId"`
	RequestId string `json:"requestId"`
}

type DeclineFriendRequestRequest struct {
	ActorId   string `json:"actorId"`
	RequestId string `json:"requestId"`
}

type RemoveFriendRequest struct {
	ActorId  string `json:"actorId"`
	FriendId string `json:"friendId"`
}

type ListFriendsRequest struct {
	UserId string `json:"userId"`
}

type ListPendingFriendRequestsRequest struct {
	UserId string `json:"userId"`
}

type FriendResponse struct {
	Id        string `json:"id"`
	UserId    string `json:"userId"`
	FriendId  string `json:"friendId"`
	CreatedAt string `json:"createdAt"`
}

type FriendRequestResponse struct {
	Id         string `json:"id"`
	SenderId   string `json:"senderId"`
	ReceiverId string `json:"receiverId"`
	CreatedAt  string `json:"createdAt"`
}

type ListFriendsResponse struct {
	Friends []*FriendResponse `json:"friends"`
}

type ListPendingFriendRequestsResponse struct {
	Requests []*FriendRequestResponse `json:"requests"`
}

type ActionResponse struct {
	Success bool `json:"success"`
}
