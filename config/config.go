package config

import (
	"flag"
	"github.com/caarlos0/env/v6"
	"net/url"
	"time"
)

type Config struct {
	BaseURL               *url.URL      `env:"BASE_URL"`
	ServerAddress         string        `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	ServerShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"5s"`
	FileStoragePath       string        `env:"FILE_STORAGE_PATH"`
}

func Configure() (*Config, error) {
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
