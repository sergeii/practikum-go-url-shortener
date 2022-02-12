package main

import (
	"flag"
	"net/url"

	"github.com/sergeii/practikum-go-url-shortener/internal/app"
)

func useFlags(cfg *app.Config) error {
	flagConfig := struct {
		BaseURL         string
		ServerAddress   string
		FileStoragePath string
		DatabaseDSN     string
	}{}
	flag.StringVar(&flagConfig.ServerAddress, "a", "", "Server listen address in the form of host:port")
	flag.StringVar(&flagConfig.BaseURL, "b", "", "Base URL for short links")
	flag.StringVar(&flagConfig.FileStoragePath, "f", "", "File path to persistent URL database storage")
	flag.StringVar(&flagConfig.DatabaseDSN, "d", "", "Database connection DSN")
	flag.Parse()
	// Указанные значения настроек из CLI-аргументов имеют преимущество перед одноименными environment переменными
	if flagConfig.BaseURL != "" {
		u, err := url.Parse(flagConfig.BaseURL)
		if err != nil {
			return err
		}
		cfg.BaseURL = u
	}
	if flagConfig.ServerAddress != "" {
		cfg.ServerAddress = flagConfig.ServerAddress
	}
	if flagConfig.FileStoragePath != "" {
		cfg.FileStoragePath = flagConfig.FileStoragePath
	}
	if flagConfig.DatabaseDSN != "" {
		cfg.DatabaseDSN = flagConfig.DatabaseDSN
	}
	return nil
}
