package shortener

import (
	"crypto/sha256"
	"encoding/hex"
)

type ShaShortener struct{}

const shortHashLength = 7

func NewShaShortener() *ShaShortener {
	return &ShaShortener{}
}

func (h ShaShortener) Shorten(s string) string {
	algo := sha256.New()
	algo.Write([]byte(s))
	longURLSha := hex.EncodeToString(algo.Sum(nil))
	return longURLSha[:shortHashLength]
}
