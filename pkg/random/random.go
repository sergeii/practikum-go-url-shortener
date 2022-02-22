package random

import (
	"math/rand"
	"time"
)

// nolint: gochecknoinits
func init() {
	rand.Seed(time.Now().UnixNano())
}

func String(length int, alphabet string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = alphabet[rand.Int63()%int64(len(alphabet))] // nolint:gosec
	}
	return string(b)
}
