package storage

import (
	"context"
	"sync"
)

type LocURLItem struct {
	LongURL   string
	UserID    string
	IsDeleted bool
}

type LocmemURLStorerBackend struct {
	Storage map[string]LocURLItem
	created map[string]string
	mu      sync.RWMutex
}

func NewLocmemURLStorerBackend() *LocmemURLStorerBackend {
	storage := make(map[string]LocURLItem)
	created := make(map[string]string)
	return &LocmemURLStorerBackend{
		Storage: storage,
		created: created,
	}
}

func (backend *LocmemURLStorerBackend) Set(ctx context.Context, shortID, longURL, userID string) (string, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	actualShortID, exists := backend.created[longURL]
	if exists {
		return actualShortID, ErrURLAlreadyExists
	}
	backend.Storage[shortID] = LocURLItem{LongURL: longURL, UserID: userID}
	backend.created[longURL] = shortID
	return shortID, nil
}

func (backend *LocmemURLStorerBackend) Get(ctx context.Context, shortURLID string) (string, error) {
	backend.mu.RLock()
	defer backend.mu.RUnlock()
	item, found := backend.Storage[shortURLID]
	if !found {
		return "", ErrURLNotFound
	} else if item.IsDeleted {
		return "", ErrURLIsDeleted
	}
	return item.LongURL, nil
}

func (backend *LocmemURLStorerBackend) GetURLsByUserID(ctx context.Context, userID string) (map[string]string, error) {
	backend.mu.RLock()
	defer backend.mu.RUnlock()
	items := make(map[string]string)
	if userID == "" {
		return items, nil
	}
	for shortURL, item := range backend.Storage {
		if item.UserID == userID && !item.IsDeleted {
			items[shortURL] = item.LongURL
		}
	}
	return items, nil
}

func (backend *LocmemURLStorerBackend) DeleteUserURLs(ctx context.Context, userID string, shortIDs ...string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	// не позволяем анонимам удалять ссылки других анонимов
	if userID == "" {
		return nil
	}
	for _, shortID := range shortIDs {
		if item, ok := backend.Storage[shortID]; ok && item.UserID == userID {
			item.IsDeleted = true
			backend.Storage[shortID] = item
			delete(backend.created, item.LongURL)
		}
	}
	return nil
}

func (backend *LocmemURLStorerBackend) SaveBatch(ctx context.Context, items []BatchItem) (map[string]string, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	result := make(map[string]string)
	for _, item := range items {
		// Проверяем на дубли
		if val, exists := backend.created[item.LongURL]; exists {
			result[item.LongURL] = val
		} else {
			backend.Storage[item.ShortID] = LocURLItem{LongURL: item.LongURL, UserID: item.UserID}
			backend.created[item.LongURL] = item.ShortID
			result[item.LongURL] = item.ShortID
		}
	}
	return result, nil
}

func (backend *LocmemURLStorerBackend) Ping(ctx context.Context) error {
	return nil
}

func (backend *LocmemURLStorerBackend) Cleanup() {
	backend.Storage = make(map[string]LocURLItem)
}

func (backend *LocmemURLStorerBackend) Close() error {
	// do nothing
	return nil
}
