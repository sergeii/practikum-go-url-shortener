package storage

type URLStorer interface {
	Set(string, string)
	Get(string) (string, error)
}
