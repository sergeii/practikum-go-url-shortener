package hasher

import (
	"crypto/sha256"
	"encoding/hex"
)

type SimpleURLHasher struct {
	Length int
}

var defaultLength = 7

func NewSimpleURLHasher() *SimpleURLHasher {
	return &SimpleURLHasher{}
}

func (hasher SimpleURLHasher) HashURL(longURL string) string {
	algo := sha256.New()
	algo.Write([]byte(longURL))
	longURLSha := hex.EncodeToString(algo.Sum(nil))
	hashLength := hasher.getLength()
	return longURLSha[:hashLength]
}

func (hasher SimpleURLHasher) getLength() int {
	if hasher.Length > 0 {
		return hasher.Length
	}
	// Значение по умолчанию
	return defaultLength
}
