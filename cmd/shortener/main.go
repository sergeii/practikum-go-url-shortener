package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sergeii/practikum-go-url-shortener/cmd/shortener/handlers"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

const (
	shutdownDur = 5000 * time.Millisecond
)

type Config struct {
	ShutdownTimeout time.Duration
}

func main() {
	handler := &handlers.URLShortenerHandler{
		Storage: storage.NewLocmemURLStorerBackend(),
		Hasher:  hasher.NewSimpleURLHasher(),
	}
	router := NewRouter(handler)

	cfg := Config{ShutdownTimeout: shutdownDur}
	server := &http.Server{
		Addr:    "localhost:8080",
		Handler: router,
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
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
		router.Post("/", handler.ShortenURL)
		router.Get("/{slug:[a-z0-9]+}", handler.ExpandURL)
	})
	return router
}
