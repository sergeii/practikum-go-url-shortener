package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sergeii/practikum-go-url-shortener/cmd/shortener/handlers"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

func main() {
	handler := &handlers.URLShortenerHandler{
		Storage: storage.NewLocmemURLStorerBackend(),
		Hasher:  &hasher.SimpleURLHasher{},
	}
	r := NewRouter(handler)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	server := &http.Server{
		Addr:    "localhost:8080",
		Handler: r,
	}
	log.Fatal(server.ListenAndServe())
}

func NewRouter(handler *handlers.URLShortenerHandler) chi.Router {
	router := chi.NewRouter()
	router.Route("/", func(r chi.Router) {
		router.Post("/", handler.ShortenURL)
		router.Get("/{slug:[a-z0-9]+}", handler.ExpandURL)
	})
	return router
}
