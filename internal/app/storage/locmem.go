package storage

import "errors"

type LocmemURLStorerBackend struct {
	Storage map[string]string
}

func NewLocmemURLStorerBackend() *LocmemURLStorerBackend {
	storage := make(map[string]string)
	return &LocmemURLStorerBackend{
		Storage: storage,
	}
}

func (backend LocmemURLStorerBackend) Set(shortURLID, longURL string) {
	backend.Storage[shortURLID] = longURL
}

func (backend LocmemURLStorerBackend) Get(shortURLID string) (string, error) {
	longURL, found := backend.Storage[shortURLID]
	if !found {
		return "", errors.New("url not found")
	}
	return longURL, nil
}
