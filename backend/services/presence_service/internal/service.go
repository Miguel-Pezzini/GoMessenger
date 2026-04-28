package presence

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrPresenceNotFound = errors.New("presence not found")

const (
	activeUserLookupConcurrency = 8
	activeUserLookupTimeout     = 2 * time.Second
)

type Service struct {
	repo           Repository
	updateChannel  string
	usernameLookup UsernameLookup
}

type UsernameLookup interface {
	LookupUsernameByUserID(ctx context.Context, userID string) (string, error)
}

func NewService(repo Repository, updateChannel string, usernameLookup ...UsernameLookup) *Service {
	var lookup UsernameLookup
	if len(usernameLookup) > 0 {
		lookup = usernameLookup[0]
	}
	return &Service{repo: repo, updateChannel: updateChannel, usernameLookup: lookup}
}

func (s *Service) HandleLifecycleEvent(ctx context.Context, event LifecycleEvent) (Presence, error) {
	if event.UserID == "" {
		return Presence{}, errors.New("user_id is required")
	}

	presence := Presence{
		UserID:        event.UserID,
		CurrentChatID: event.CurrentChatID,
	}

	switch event.Type {
	case LifecycleEventConnected:
		presence.Status = StatusOnline
	case LifecycleEventChatOpened:
		if event.CurrentChatID == "" {
			return Presence{}, errors.New("current_chat_id is required for chat_opened events")
		}
		presence.Status = StatusOnline
	case LifecycleEventChatClosed:
		presence.Status = StatusOnline
		presence.CurrentChatID = ""
	case LifecycleEventDisconnected:
		presence.Status = StatusOffline
		presence.CurrentChatID = ""
		occurredAt := event.OccurredAt
		if occurredAt.IsZero() {
			return Presence{}, errors.New("occurred_at is required for disconnect events")
		}
		presence.LastSeen = &occurredAt
	default:
		return Presence{}, errors.New("unsupported lifecycle event type")
	}

	if err := s.repo.Save(ctx, presence); err != nil {
		return Presence{}, err
	}
	if err := s.repo.Publish(ctx, s.updateChannel, presence); err != nil {
		return Presence{}, err
	}

	return presence, nil
}

func (s *Service) GetPresence(ctx context.Context, userID string) (Presence, error) {
	if userID == "" {
		return Presence{}, errors.New("user_id is required")
	}

	return s.repo.Get(ctx, userID)
}

func (s *Service) ListActiveUsers(ctx context.Context, limit int) (ActiveUsersResponse, error) {
	presences, err := s.repo.ListActive(ctx, limit)
	if err != nil {
		return ActiveUsersResponse{}, err
	}

	users := make([]ActiveUser, len(presences))
	for i, presence := range presences {
		users[i] = ActiveUser{
			UserID:        presence.UserID,
			Status:        presence.Status,
			LastSeen:      presence.LastSeen,
			CurrentChatID: presence.CurrentChatID,
		}
	}

	if s.usernameLookup == nil || len(presences) == 0 {
		return ActiveUsersResponse{Users: users, Count: len(users)}, nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, activeUserLookupTimeout)
	defer cancel()

	sem := make(chan struct{}, activeUserLookupConcurrency)
	var wg sync.WaitGroup

launchLoop:
	for i, presence := range presences {
		select {
		case sem <- struct{}{}:
		case <-lookupCtx.Done():
			break launchLoop
		}

		wg.Add(1)
		go func(index int, userID string) {
			defer wg.Done()
			defer func() { <-sem }()

			username, err := s.usernameLookup.LookupUsernameByUserID(lookupCtx, userID)
			if err == nil {
				users[index].Username = username
			}
		}(i, presence.UserID)
	}

	wg.Wait()

	return ActiveUsersResponse{Users: users, Count: len(users)}, nil
}
