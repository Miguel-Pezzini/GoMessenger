package friends

type Friend struct {
	ID        string `json:"id"`
	OwnerID   string `json:"ownerId"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type CreateFriendRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

type UpdateFriendRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}
