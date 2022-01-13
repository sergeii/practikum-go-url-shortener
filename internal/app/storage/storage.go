package storage

import "errors"

var URLNotFound = errors.New("URL not found in the storage")

type URLStorer interface {
	Set(string, string)
	Get(string) (string, error)
}
