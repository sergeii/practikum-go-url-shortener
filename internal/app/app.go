package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/caarlos0/env/v6"

	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/sergeii/practikum-go-url-shortener/pkg/url/hasher"
	"github.com/sergeii/practikum-go-url-shortener/storage"
)

const SecretKeyLength = 32

type Config struct {
	BaseURL               *url.URL      `env:"BASE_URL"`
	ServerAddress         string        `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	ServerShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"5s"`
	FileStoragePath       string        `env:"FILE_STORAGE_PATH"`
	SecretKey             string        `env:"SECRET_KEY"`
	DatabaseDSN           string        `env:"DATABASE_DSN" envDefault:"postgres://postgres:postgres@postgres:5432/praktikum"` // nolint:lll
	DatabasePingTimeout   time.Duration `env:"DATABASE_PING_TIMEOUT" envDefault:"5s"`
}

type App struct {
	Config    *Config
	Storage   storage.URLStorer
	Hasher    hasher.Hasher
	DB        *pgxpool.Pool
	SecretKey []byte
}

type Override func(*Config) error

func New(overrides ...Override) (*App, error) {
	var cfg Config
	// Получаем настройки приложения из environment-переменных
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	// даем возможность переопределить настройки, например в тестах или при использовании флагов
	for _, override := range overrides {
		if err := override(&cfg); err != nil {
			return nil, err
		}
	}

	store, err := configureStorage(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to configure storage due to %w", err)
	}

	secretKey, err := configureSecretKey(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to configure secret key due to %w", err)
	}

	db, err := ConfigureDatabase(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to configure database due to %w", err)
	}

	app := &App{
		Storage:   store,
		Hasher:    hasher.NewNaiveHasher(),
		Config:    &cfg,
		DB:        db,
		SecretKey: secretKey,
	}
	return app, nil
}

func (app *App) Close() {
	if err := app.Storage.Close(); err != nil {
		log.Printf("failed to close storage %s due to %s; possible data loss", app.Storage, err)
	}
	app.DB.Close()
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
	if _, err := rand.Read(randKey); err != nil {
		return nil, err
	}
	return randKey, nil
}

func ConfigureDatabase(cfg *Config) (*pgxpool.Pool, error) {
	return pgxpool.Connect(context.Background(), cfg.DatabaseDSN)
}
