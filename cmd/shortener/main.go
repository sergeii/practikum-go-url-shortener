package main

import (
	"context"
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
	BaseURL               url.URL       `env:"BASE_URL"`
	ServerAddress         string        `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	ServerShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"5s"`
}

func main() {
	var cfg Config

	// Парсим настройки сервиса, используя переменные окружения
	if err := env.Parse(&cfg); err != nil {
		log.Fatal(err)
		return
	}

	handler := &handlers.URLShortenerHandler{
		Storage: storage.NewLocmemURLStorerBackend(),
		Hasher:  hasher.NewSimpleURLHasher(),
		BaseURL: cfg.BaseURL,
	}
	router := NewRouter(handler)

	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: router,
	}

	// Запускаем сервер и при необходимости останавливаем его gracefully
	startStopServer(server, &cfg)
}

func startStopServer(server *http.Server, cfg *Config) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to listen and serve due to: %s\n", err)
		}
	}()
	log.Printf("Server started at %s", server.Addr)

	<-sigint
	log.Print("Stopping the server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ServerShutdownTimeout)
	defer func() {
		cancel()
	}()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed due to: %s\n", err)
	}
	log.Print("Stopped the server successfully")
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
