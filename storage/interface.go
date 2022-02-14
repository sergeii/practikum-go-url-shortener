package storage

import "context"

type URLStorer interface {
	Set(context.Context, string, string, string) (string, error)
	Get(context.Context, string) (string, error)
	GetURLsByUserID(context.Context, string) (map[string]string, error)
	SaveBatch(context.Context, []BatchItem) error
	Cleanup()
	Close() error
}
