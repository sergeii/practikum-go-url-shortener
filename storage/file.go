package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
)

type FileURLItem struct {
	LongURL string
	UserID  string
}

type FileURLStorerBackend struct {
	filename string
	cache    map[string]FileURLItem
}

func NewFileURLStorerBackend(filename string) (*FileURLStorerBackend, error) {
	cache := make(map[string]FileURLItem)
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
		}
	}
	return &FileURLStorerBackend{filename, cache}, nil
}

func (backend FileURLStorerBackend) Set(ctx context.Context, shortURLID, longURL, userID string) error {
	backend.cache[shortURLID] = FileURLItem{longURL, userID}
	return nil
}

func (backend FileURLStorerBackend) Get(ctx context.Context, shortURLID string) (string, error) {
	item, found := backend.cache[shortURLID]
	if !found {
		return "", ErrURLNotFound
	}
	return item.LongURL, nil
}

func (backend FileURLStorerBackend) GetURLsByUserID(ctx context.Context, userID string) (map[string]string, error) {
	items := make(map[string]string)
	if userID == "" {
		return items, nil
	}
	for shortURL, item := range backend.cache {
		if item.UserID == userID {
			items[shortURL] = item.LongURL
		}
	}
	return items, nil
}

func (backend FileURLStorerBackend) Close() error {
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
