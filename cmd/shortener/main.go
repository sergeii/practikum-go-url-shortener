package main

import (
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/router"
	"github.com/sergeii/practikum-go-url-shortener/pkg/http/server"
	"log"
	"net/http"
)

func main() {
	// Собираем настройки сервиса из аргументов командной строки и переменных окружения
	cfg, err := ConfigureSettings()
	if err != nil {
		log.Fatalf("failed to init config due to %s\n", err)
	}

	shortener, err := app.New(cfg)
	if err != nil {
		log.Fatalf("failed to init app due to %s\n", err)
	}
	defer func() {
		if err := shortener.Close(); err != nil {
			log.Printf("failed to clean up app due to %s\n", err)
		}
	}()

	rtr := router.New(shortener)
	svr := &http.Server{
		Addr:    shortener.Config.ServerAddress,
		Handler: rtr,
	}
	err = server.Start(svr, server.WithShutdownTimeout(shortener.Config.ServerShutdownTimeout))
	if err != nil {
		log.Fatalf("Server exited prematurely: %s\n", err)
	}
}
