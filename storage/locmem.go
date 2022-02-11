package storage

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

func (backend LocmemURLStorerBackend) Set(shortURLID, longURL, userID string) {
	backend.Storage[shortURLID] = LocURLItem{longURL, userID}
}

func (backend LocmemURLStorerBackend) Get(shortURLID string) (string, error) {
	item, found := backend.Storage[shortURLID]
	if !found {
		return "", ErrURLNotFound
	}
	return item.LongURL, nil
}

func (backend LocmemURLStorerBackend) GetURLsByUserID(userID string) map[string]string {
	items := make(map[string]string)
	if userID == "" {
		return items
	}
	for shortURL, item := range backend.Storage {
		if item.UserID == userID {
			items[shortURL] = item.LongURL
		}
	}
	return items
}

func (backend LocmemURLStorerBackend) Close() error {
	// do nothing
	return nil
}
