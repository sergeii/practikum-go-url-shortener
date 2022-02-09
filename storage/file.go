package storage

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
)

type FileURLStorerBackend struct {
	filename string
	cache    map[string]string
}

func NewFileURLStorerBackend(filename string) (*FileURLStorerBackend, error) {
	cache := make(map[string]string)
	// Считываем с диска записи, сохраненные ранее, и заполняем ими кэш,
	// с которым мы и будем работать до завершения программы
	file, err := os.OpenFile(filename, os.O_RDONLY, 0777)
	if err != nil {
		// Если файл не найден, то ничего страшного - это ожидаемое поведение при первом запуске сервиса
		if os.IsNotExist(err) {
			log.Printf("file %s not found; will start with empty cache\n", filename)
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
				log.Printf("file is empty %s; will start with empty cache\n", filename)
			} else {
				log.Printf("unable to populate cache from %s due to %s\n", filename, err)
				return nil, err
			}
		}
	}

	return &FileURLStorerBackend{
		filename: filename,
		cache:    cache,
	}, nil
}

func (backend FileURLStorerBackend) Set(shortURLID, longURL string) {
	backend.cache[shortURLID] = longURL
}

func (backend FileURLStorerBackend) Get(shortURLID string) (string, error) {
	longURL, found := backend.cache[shortURLID]
	if !found {
		return "", ErrURLNotFound
	}
	return longURL, nil
}

func (backend FileURLStorerBackend) Close() error {
	// Сохраняем на диск рабочий кэш со ссылками,
	// который будет использован при следующем старте программы
	file, err := os.OpenFile(backend.filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		log.Printf("unable to open file %s for dumping cache due to %s\n", backend.filename, err)
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(&backend.cache); err != nil {
		log.Printf("unable to dump cache to %s due to %s\n", backend.filename, err)
		return err
	}
	return nil
}
