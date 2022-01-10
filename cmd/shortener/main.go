package main

import (
	"log"
	"net/http"

	"github.com/sergeii/practikum-go-url-shortener/cmd/shortener/handlers"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

func main() {
	server := &http.Server{
		Addr: "localhost:8080",
		Handler: &handlers.URLShortenerHandler{
			Storage: storage.NewLocmemURLStorerBackend(),
			Hasher:  &hasher.SimpleURLHasher{},
		},
	}
	log.Fatal(server.ListenAndServe())
}
