package storage

type URLStorer interface {
	Set(string, string, string)
	Get(string) (string, error)
	GetURLsByUserID(string) map[string]string
	Close() error
}
