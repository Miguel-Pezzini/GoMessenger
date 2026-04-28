package friends

import (
	"context"
	"errors"
	"testing"
)

type lookupStub struct {
	id  string
	err error
}

func (s lookupStub) LookupUserIDByFriendCode(context.Context, string) (string, error) {
	return s.id, s.err
}

func TestResolveReceiverUserIDFriendCode(t *testing.T) {
	id, err := ResolveReceiverUserID(context.Background(), "12345678", lookupStub{id: "abc123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "abc123" {
		t.Fatalf("expected abc123, got %s", id)
	}
}

func TestResolveReceiverUserIDFriendCodeUnknown(t *testing.T) {
	_, err := ResolveReceiverUserID(context.Background(), "87654321", lookupStub{err: ErrUnknownFriendCode})
	if !errors.Is(err, ErrUnknownFriendCode) {
		t.Fatalf("expected ErrUnknownFriendCode, got %v", err)
	}
}

func TestResolveReceiverUserIDFriendCodeLookupNil(t *testing.T) {
	_, err := ResolveReceiverUserID(context.Background(), "12345678", nil)
	if !errors.Is(err, ErrFriendLookupUnavailable) {
		t.Fatalf("expected ErrFriendLookupUnavailable, got %v", err)
	}
}

func TestResolveReceiverUserIDHexObjectID(t *testing.T) {
	id, err := ResolveReceiverUserID(context.Background(), "507f1f77bcf86cd799439011", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "507f1f77bcf86cd799439011" {
		t.Fatalf("unexpected id %s", id)
	}
}

func TestResolveReceiverUserIDPassthrough(t *testing.T) {
	id, err := ResolveReceiverUserID(context.Background(), "user-2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "user-2" {
		t.Fatalf("expected passthrough, got %s", id)
	}
}

func TestResolveReceiverUserIDEmpty(t *testing.T) {
	_, err := ResolveReceiverUserID(context.Background(), "   ", nil)
	if !errors.Is(err, ErrInvalidReceiverID) {
		t.Fatalf("expected ErrInvalidReceiverID, got %v", err)
	}
}
