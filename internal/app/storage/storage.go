package storage

type UrlStorer interface {
	Set(string, string)
	Get(string) (string, error)
}
