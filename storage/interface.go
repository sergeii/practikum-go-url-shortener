package storage

import "context"

type URLStorer interface {
	Set(context.Context, string, string, string) (string, error)
	Get(context.Context, string) (string, error)
	GetURLsByUserID(context.Context, string) (map[string]string, error)
	DeleteUserURLs(context.Context, string, ...string) error
	SaveBatch(context.Context, []BatchItem) (map[string]string, error)
	Ping(context.Context) error
	Cleanup()
	Close() error
}
