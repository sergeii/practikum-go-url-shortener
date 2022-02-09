package app

import (
	"fmt"
	"github.com/sergeii/practikum-go-url-shortener/config"
	"github.com/sergeii/practikum-go-url-shortener/pkg/url/hasher"
	"github.com/sergeii/practikum-go-url-shortener/storage"
)

type App struct {
	Config  *config.Config
	Storage storage.URLStorer
	Hasher  hasher.Hasher
}

func New(cfg *config.Config) (*App, error) {
	store, err := configureStorage(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to init and configure storage due to %s", err)
	}
	app := &App{
		Storage: store,
		Hasher:  hasher.NewNaiveHasher(),
		Config:  cfg,
	}
	return app, nil
}

func (app *App) Close() error {
	if err := app.Storage.Close(); err != nil {
		return fmt.Errorf("failed to close storage %s due to %s; possible data loss", app.Storage, err)
	}
	return nil
}

// configureStorage инициализирует тип хранилища
// в зависимости от настроек сервиса, заданных переменными окружения
func configureStorage(cfg *config.Config) (storage.URLStorer, error) {
	if cfg.FileStoragePath != "" {
		return storage.NewFileURLStorerBackend(cfg.FileStoragePath)
	}
	return storage.NewLocmemURLStorerBackend(), nil
}
