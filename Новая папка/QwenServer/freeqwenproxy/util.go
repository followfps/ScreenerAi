package freeqwenproxy

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

func newHexID(nBytes int) string {
	if nBytes <= 0 {
		nBytes = 16
	}
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func normalizeAPIBaseURL(s string) string {
	u := strings.TrimSpace(s)
	u = strings.TrimRight(u, "/")
	return u
}

