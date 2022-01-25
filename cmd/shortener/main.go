package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sergeii/practikum-go-url-shortener/cmd/shortener/handlers"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

type Config struct {
	BaseURL               *url.URL      `env:"BASE_URL"`
	ServerAddress         string        `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	ServerShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"5s"`
	FileStoragePath       string        `env:"FILE_STORAGE_PATH"`
}

func main() {
	// Собираем настройки сервиса из аргументов командной строки и переменных окружения
	cfg, err := configureSettings()
	if err != nil {
		log.Fatal(err)
		return
	}

	URLStorage, err := configureStorage(cfg)
	if err != nil {
		log.Fatalf("unable to init and configure storage due to %s\n", err)
		return
	}
	defer func() {
		if err := URLStorage.Close(); err != nil {
			log.Fatalf("failed to close storage %s due to %s; possible data loss\n", URLStorage, err)
		}
	}()

	handler := &handlers.URLShortenerHandler{
		Storage: URLStorage,
		Hasher:  hasher.NewSimpleURLHasher(),
		BaseURL: cfg.BaseURL,
	}
	router := NewRouter(handler)

	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: router,
	}

	// Запускаем сервер и при необходимости останавливаем его gracefully
	startStopServer(server, cfg)
}

func startStopServer(server *http.Server, cfg *Config) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to listen and serve due to: %s\n", err)
		}
	}()
	log.Printf("Server started at %s with settings:\n%+v\n", server.Addr, cfg)

	<-sigint
	log.Print("Stopping the server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ServerShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed due to: %s\n", err)
	}
	log.Print("Stopped the server successfully")
}

func configureSettings() (*Config, error) {
	var cfg Config

	// Парсим настройки сервиса, используя как переменные окружения...
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	// ..так и CLI-аргументы
	flagConfig := struct {
		BaseURL         string
		ServerAddress   string
		FileStoragePath string
	}{}
	flag.StringVar(&flagConfig.ServerAddress, "a", "", "Server listen address in the form of host:port")
	flag.StringVar(&flagConfig.BaseURL, "b", "", "Base URL for short links")
	flag.StringVar(&flagConfig.FileStoragePath, "f", "", "File path to persistent URL database storage")
	flag.Parse()
	// Указанные значения настроек из CLI-аргументов имеют преимущество перед одноименными environment переменными
	if flagConfig.BaseURL != "" {
		u, err := url.Parse(flagConfig.BaseURL)
		if err != nil {
			return nil, err
		}
		cfg.BaseURL = u
	}
	if flagConfig.ServerAddress != "" {
		cfg.ServerAddress = flagConfig.ServerAddress
	}
	if flagConfig.FileStoragePath != "" {
		cfg.FileStoragePath = flagConfig.FileStoragePath
	}

	return &cfg, nil
}

// configureStorage инициализирует тип хранилища
// в зависимости от настроек сервиса, заданных переменными окружения
func configureStorage(cfg *Config) (storage.URLStorer, error) {
	if cfg.FileStoragePath != "" {
		return storage.NewFileURLStorerBackend(cfg.FileStoragePath)
	}
	return storage.NewLocmemURLStorerBackend(), nil
}

func NewRouter(handler *handlers.URLShortenerHandler) chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Route("/", func(r chi.Router) {
		r.Post("/", handler.ShortenURL)
		r.Get("/{slug:[a-z0-9]+}", handler.ExpandURL)
	})
	router.Route("/api", func(r chi.Router) {
		r.Post("/shorten", handler.APIShortenURL)
	})
	return router
}
