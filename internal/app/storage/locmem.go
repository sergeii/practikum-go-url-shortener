package storage

import "errors"

type LocmemURLStorerBackend struct {
	cache map[string]string
}

func NewLocmemURLShortenerBackend() *LocmemURLStorerBackend {
	storage := map[string]string{
		"go": "https://go.dev/", // Для тестирования
	}
	return &LocmemURLStorerBackend{
		cache: storage,
	}
}

func (backend LocmemURLStorerBackend) Set(shortURLID, longURL string) {
	backend.cache[shortURLID] = longURL
}

func (backend LocmemURLStorerBackend) Get(shortURLID string) (string, error) {
	longURL, found := backend.cache[shortURLID]
	if !found {
		return "", errors.New("url not found")
	}
	return longURL, nil
}
