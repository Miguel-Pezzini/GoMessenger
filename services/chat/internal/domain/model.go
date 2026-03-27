package domain

type MessageRequest struct {
	StreamID   string `json:"-"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

type MessageDB struct {
	Id         string `json:"id" bson:"-"`
	StreamID   string `json:"-" bson:"stream_id"`
	SenderID   string `json:"sender_id" bson:"sender_id"`
	ReceiverID string `json:"receiver_id" bson:"receiver_id"`
	Content    string `json:"content" bson:"content"`
	Timestamp  int64  `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
}

type MessageResponse struct {
	Id         string `json:"id"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}
