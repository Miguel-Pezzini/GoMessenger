package friends

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Publisher struct {
	client                   *goredis.Client
	friendNotificationStream string
}

type FriendRequestNotificationIntent struct {
	Type            string `json:"type"`
	EventID         string `json:"event_id"`
	SenderID        string `json:"sender_id"`
	ReceiverID      string `json:"receiver_id"`
	FriendRequestID string `json:"friend_request_id"`
	OccurredAt      string `json:"occurred_at"`
}

func NewPublisher(client *goredis.Client, friendNotificationStream string) *Publisher {
	return &Publisher{client: client, friendNotificationStream: friendNotificationStream}
}

func (p *Publisher) Publish(channel string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = p.client.Publish(context.Background(), channel, data).Err()
}

func (p *Publisher) PublishFriendRequestNotificationIntent(senderID, receiverID, friendRequestID string) error {
	if p.friendNotificationStream == "" {
		return nil
	}

	intent := FriendRequestNotificationIntent{
		Type:            "friend_request_received",
		EventID:         friendRequestID,
		SenderID:        senderID,
		ReceiverID:      receiverID,
		FriendRequestID: friendRequestID,
		OccurredAt:      time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(intent)
	if err != nil {
		return err
	}

	_, err = p.client.XAdd(context.Background(), &goredis.XAddArgs{
		Stream: p.friendNotificationStream,
		Values: map[string]any{"payload": string(data)},
	}).Result()
	return err
}
