package storage

import "errors"

type LocmemUrlStorerBackend struct {
	cache map[string]string
}

func NewLocmemUrlShortenerBackend() *LocmemUrlStorerBackend {
	storage := map[string]string{
		"go": "https://go.dev/", // Для тестирования
	}
	return &LocmemUrlStorerBackend{
		cache: storage,
	}
}

func (backend LocmemUrlStorerBackend) Set(shortUrlId, longUrl string) {
	backend.cache[shortUrlId] = longUrl
}

func (backend LocmemUrlStorerBackend) Get(shortUrlId string) (string, error) {
	longUrl, found := backend.cache[shortUrlId]
	if !found {
		return "", errors.New("url not found")
	}
	return longUrl, nil
}
