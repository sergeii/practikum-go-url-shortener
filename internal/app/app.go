package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/sergeii/practikum-go-url-shortener/pkg/url/hasher"
	"github.com/sergeii/practikum-go-url-shortener/storage"
	"net/url"
	"time"
)

const SecretKeyLength = 32

type Config struct {
	BaseURL               *url.URL      `env:"BASE_URL"`
	ServerAddress         string        `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	ServerShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"5s"`
	FileStoragePath       string        `env:"FILE_STORAGE_PATH"`
	SecretKey             string        `env:"SECRET_KEY"`
}

type App struct {
	Config    *Config
	Storage   storage.URLStorer
	Hasher    hasher.Hasher
	SecretKey []byte
}

func New(cfg *Config) (*App, error) {
	store, err := configureStorage(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to configure storage due to %s", err)
	}
	secretKey, err := configureSecretKey(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to configure secret key due to %s", err)
	}
	app := &App{
		Storage:   store,
		Hasher:    hasher.NewNaiveHasher(),
		Config:    cfg,
		SecretKey: secretKey,
	}
	return app, nil
}

func (app *App) Close() error {
	if err := app.Storage.Close(); err != nil {
		return fmt.Errorf("failed to close storage %s due to %s; possible data loss", app.Storage, err)
	}
	return nil
}

// configureStorage инициализирует тип хранилища
// в зависимости от настроек сервиса, заданных переменными окружения
func configureStorage(cfg *Config) (storage.URLStorer, error) {
	if cfg.FileStoragePath != "" {
		return storage.NewFileURLStorerBackend(cfg.FileStoragePath)
	}
	return storage.NewLocmemURLStorerBackend(), nil
}

// configureSecretKey декодирует в слайс байт секретный ключ приложения,
// установленный environment переменной в виде hex-строки
// В случае отсутствия ключа, его значение генерируется рандомно
func configureSecretKey(cfg *Config) ([]byte, error) {
	if cfg.SecretKey != "" {
		confKey, err := hex.DecodeString(cfg.SecretKey)
		if err != nil {
			return nil, err
		}
		return confKey, nil
	}
	return GenerateSecretKey(SecretKeyLength)
}

func GenerateSecretKey(length int) ([]byte, error) {
	randKey := make([]byte, length)
	_, err := rand.Read(randKey)
	if err != nil {
		return nil, err
	}
	return randKey, nil
}
