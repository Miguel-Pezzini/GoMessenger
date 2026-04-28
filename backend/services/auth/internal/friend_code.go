package auth

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

const friendCodeDigitCount = 8

func generateFriendCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	// 8 digits: 10_000_000 .. 99_999_999
	n := binary.BigEndian.Uint32(buf[:])
	v := 10_000_000 + n%90_000_000
	return fmt.Sprintf("%08d", v), nil
}
