package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"sync"
)

type FileURLItem struct {
	LongURL   string
	UserID    string
	IsDeleted bool
}

type FileURLStorerBackend struct {
	filename string
	cache    map[string]FileURLItem
	created  map[string]string
	mu       sync.RWMutex
}

func NewFileURLStorerBackend(filename string) (*FileURLStorerBackend, error) {
	cache := make(map[string]FileURLItem)
	created := make(map[string]string) // служебная мапа, хранящая ссылки в формате URL -> Short ID
	// Считываем с диска записи, сохраненные ранее, и заполняем ими кэш,
	// с которым мы и будем работать до завершения программы
	file, err := os.OpenFile(filename, os.O_RDONLY, 0777)
	if err != nil {
		// Если файл не найден, то ничего страшного - это ожидаемое поведение при первом запуске сервиса
		if os.IsNotExist(err) {
			log.Printf("file %s not found; will start with empty Storage\n", filename)
		} else {
			log.Printf("error opening %s: %s\n", filename, err)
			return nil, err
		}
	} else {
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(&cache); err != nil {
			// Файл пустой - ожидаемое поведение
			if errors.Is(err, io.EOF) {
				log.Printf("file is empty %s; will start with empty Storage\n", filename)
			} else {
				log.Printf("unable to populate Storage from %s due to %s\n", filename, err)
				return nil, err
			}
		} else {
			// заполняем обратную мапу URL -> ShortID для быстрого поиска дублей
			for shortID, item := range cache {
				created[item.LongURL] = shortID
			}
		}
	}
	backend := FileURLStorerBackend{
		filename: filename,
		cache:    cache,
		created:  created,
	}
	return &backend, nil
}

func (backend *FileURLStorerBackend) Set(ctx context.Context, shortID, longURL, userID string) (string, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	actualShortID, exists := backend.created[longURL]
	if exists {
		return actualShortID, ErrURLAlreadyExists
	}
	backend.cache[shortID] = FileURLItem{LongURL: longURL, UserID: userID}
	backend.created[longURL] = shortID
	return shortID, nil
}

func (backend *FileURLStorerBackend) Get(ctx context.Context, shortURLID string) (string, error) {
	backend.mu.RLock()
	defer backend.mu.RUnlock()
	item, found := backend.cache[shortURLID]
	if !found {
		return "", ErrURLNotFound
	} else if item.IsDeleted {
		return "", ErrURLIsDeleted
	}
	return item.LongURL, nil
}

func (backend *FileURLStorerBackend) GetURLsByUserID(ctx context.Context, userID string) (map[string]string, error) {
	backend.mu.RLock()
	defer backend.mu.RUnlock()
	items := make(map[string]string)
	if userID == "" {
		return items, nil
	}
	for shortURL, item := range backend.cache {
		if item.UserID == userID && !item.IsDeleted {
			items[shortURL] = item.LongURL
		}
	}
	return items, nil
}

func (backend *FileURLStorerBackend) DeleteUserURLs(ctx context.Context, userID string, shortIDs ...string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	// не позволяем анонимам удалять ссылки других анонимов
	if userID == "" {
		return nil
	}
	for _, shortID := range shortIDs {
		if item, ok := backend.cache[shortID]; ok && item.UserID == userID {
			item.IsDeleted = true
			backend.cache[shortID] = item
			delete(backend.created, item.LongURL)
		}
	}
	return nil
}

func (backend *FileURLStorerBackend) SaveBatch(ctx context.Context, items []BatchItem) (map[string]string, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	result := make(map[string]string)
	for _, item := range items {
		// Проверяем на дубли
		if val, exists := backend.created[item.LongURL]; exists {
			result[item.LongURL] = val
		} else {
			backend.cache[item.ShortID] = FileURLItem{LongURL: item.LongURL, UserID: item.UserID}
			backend.created[item.LongURL] = item.ShortID
			result[item.LongURL] = item.ShortID
		}
	}
	return result, nil
}

func (backend *FileURLStorerBackend) Ping(ctx context.Context) error {
	return nil
}

func (backend *FileURLStorerBackend) Cleanup() {
	if err := os.Remove(backend.filename); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
	}
}

func (backend *FileURLStorerBackend) Close() error {
	// Сохраняем на диск рабочий кэш со ссылками,
	// который будет использован при следующем старте программы
	file, err := os.OpenFile(backend.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		log.Printf("unable to open file %s for dumping Storage due to %s\n", backend.filename, err)
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(&backend.cache); err != nil {
		log.Printf("unable to dump Storage to %s due to %s\n", backend.filename, err)
		return err
	}
	return nil
}
