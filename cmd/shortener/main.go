package main

import (
	"log"
	"net/http"

	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/router"
	"github.com/sergeii/practikum-go-url-shortener/pkg/http/server"
)

func main() {
	shortener, err := app.New(useFlags)
	if err != nil {
		log.Fatalf("failed to init app due to %s\n", err)
	}
	defer shortener.Close()

	rtr := router.New(shortener)
	svr := &http.Server{
		Addr:    shortener.Config.ServerAddress,
		Handler: rtr,
	}
	err = server.Start(svr, server.WithShutdownTimeout(shortener.Config.ServerShutdownTimeout))
	if err != nil {
		log.Printf("Server exited prematurely: %s\n", err)
	}
}
