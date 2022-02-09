package hasher

import (
	"crypto/sha256"
	"encoding/hex"
)

type NaiveHasher struct{}

const shortHashLength = 7

func NewNaiveHasher() *NaiveHasher {
	return &NaiveHasher{}
}

func (h NaiveHasher) Hash(s string) string {
	algo := sha256.New()
	algo.Write([]byte(s))
	longURLSha := hex.EncodeToString(algo.Sum(nil))
	return longURLSha[:shortHashLength]
}
