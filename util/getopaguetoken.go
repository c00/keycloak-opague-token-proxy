package util

import (
	"crypto/rand"
	"encoding/hex"
)

func GetOpagueToken(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:n], nil
}
