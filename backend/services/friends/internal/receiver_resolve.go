package friends

import (
	"context"
	"regexp"
	"strings"
)

var (
	friendCodeDigitsPattern = regexp.MustCompile(`^\d{6,8}$`)
	objectIDHexPattern      = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)
)

// FriendCodeLookup resolves a numeric friend code to the canonical user id (Mongo ObjectID hex).
type FriendCodeLookup interface {
	LookupUserIDByFriendCode(ctx context.Context, code string) (string, error)
}

// ResolveReceiverUserID maps UI input to the internal receiver user id.
// - 6–8 digit strings are treated as friend codes (via lookup).
// - 24-character hex strings are treated as legacy Mongo user ids.
// - Any other non-empty string is passed through (for tests and transitional inputs).
func ResolveReceiverUserID(ctx context.Context, raw string, lookup FriendCodeLookup) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrInvalidReceiverID
	}

	if friendCodeDigitsPattern.MatchString(s) {
		if lookup == nil {
			return "", ErrFriendLookupUnavailable
		}
		id, err := lookup.LookupUserIDByFriendCode(ctx, s)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(id), nil
	}

	if objectIDHexPattern.MatchString(s) {
		return strings.ToLower(s), nil
	}

	return s, nil
}
