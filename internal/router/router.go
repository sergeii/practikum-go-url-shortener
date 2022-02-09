package router

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/handlers"
)

func New(theApp *app.App) chi.Router {
	handler := &handlers.Handler{
		App: theApp,
	}
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
