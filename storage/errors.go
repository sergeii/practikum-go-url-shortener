package storage

import (
	"errors"
)

var ErrURLNotFound = errors.New("URL not found in the storage")
var ErrURLAlreadyExists = errors.New("URL already exists in the storage")
