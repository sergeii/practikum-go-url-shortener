package hasher

import (
	"crypto/sha256"
	"encoding/hex"
)

type SimpleUrlHasher struct {
	Length int
}

var defaultLength = 7

func (hasher SimpleUrlHasher) HashUrl(longUrl string) string {
	algo := sha256.New()
	algo.Write([]byte(longUrl))
	longUrlSha := hex.EncodeToString(algo.Sum(nil))
	hashLength := hasher.getLength()
	return longUrlSha[:hashLength]
}

func (hasher SimpleUrlHasher) getLength() int {
	if hasher.Length > 0 {
		return hasher.Length
	}
	// Значение по умолчанию
	return defaultLength
}
