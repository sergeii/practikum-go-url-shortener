package shortener

import (
	"github.com/sergeii/practikum-go-url-shortener/pkg/random"
)

type RandShortener struct{}

const randLength = 11
const alphabet = "-_0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func NewRandShortener() *RandShortener {
	return &RandShortener{}
}

func (h RandShortener) Shorten(s string) string {
	return random.String(randLength, alphabet)
}
