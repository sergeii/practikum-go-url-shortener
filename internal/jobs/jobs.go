package jobs

import (
	"context"

	"github.com/sergeii/practikum-go-url-shortener/pkg/background"
	"github.com/sergeii/practikum-go-url-shortener/storage"
)

func DeleteUserURLs(store storage.URLStorer, userID string, shortIDs ...string) background.Job {
	return background.NewJob("delete user URLs", func(ctx context.Context) error {
		return store.DeleteUserURLs(ctx, userID, shortIDs...)
	})
}
