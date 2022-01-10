package main

import (
	"github.com/go-chi/chi/v5/middleware"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sergeii/practikum-go-url-shortener/cmd/shortener/handlers"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

func main() {
	handler := &handlers.URLShortenerHandler{
		Storage: storage.NewLocmemURLStorerBackend(),
		Hasher:  &hasher.SimpleURLHasher{},
	}
	router := NewRouter(handler)
	server := &http.Server{
		Addr:    "localhost:8080",
		Handler: router,
	}
	log.Fatal(server.ListenAndServe())
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
