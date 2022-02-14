package storage

import "context"

type LocURLItem struct {
	LongURL string
	UserID  string
}

type LocmemURLStorerBackend struct {
	Storage map[string]LocURLItem
}

func NewLocmemURLStorerBackend() *LocmemURLStorerBackend {
	storage := make(map[string]LocURLItem)
	return &LocmemURLStorerBackend{Storage: storage}
}

func (backend LocmemURLStorerBackend) Set(ctx context.Context, shortURLID, longURL, userID string) error {
	backend.Storage[shortURLID] = LocURLItem{longURL, userID}
	return nil
}

func (backend LocmemURLStorerBackend) Get(ctx context.Context, shortURLID string) (string, error) {
	item, found := backend.Storage[shortURLID]
	if !found {
		return "", ErrURLNotFound
	}
	return item.LongURL, nil
}

func (backend LocmemURLStorerBackend) GetURLsByUserID(ctx context.Context, userID string) (map[string]string, error) {
	items := make(map[string]string)
	if userID == "" {
		return items, nil
	}
	for shortURL, item := range backend.Storage {
		if item.UserID == userID {
			items[shortURL] = item.LongURL
		}
	}
	return items, nil
}

func (backend LocmemURLStorerBackend) SaveBatch(ctx context.Context, items []BatchItem) error {
	return nil
}

func (backend LocmemURLStorerBackend) Cleanup() {
	// do nothing
}

func (backend LocmemURLStorerBackend) Close() error {
	// do nothing
	return nil
}
