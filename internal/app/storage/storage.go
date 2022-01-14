package storage

import "errors"

var ErrURLNotFound = errors.New("URL not found in the storage")

type URLStorer interface {
	Set(string, string)
	Get(string) (string, error)
}
