package storage

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
		return "", ErrURLNotFound
	}
	return longURL, nil
}
