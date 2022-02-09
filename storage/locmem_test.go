package storage_test

import (
	"testing"

	"github.com/sergeii/practikum-go-url-shortener/storage"
	"github.com/stretchr/testify/assert"
)

func TestSaveURLToStorage(t *testing.T) {
	theStorage := storage.NewLocmemURLStorerBackend()

	theStorage.Set("foo", "https://practicum.yandex.ru/")
	assert.Equal(t, "https://practicum.yandex.ru/", theStorage.Storage["foo"])
	// Можем перезаписать
	theStorage.Set("foo", "https://go.dev/")
	assert.Equal(t, "https://go.dev/", theStorage.Storage["foo"])

	// Или записать с другим id
	theStorage.Set("bar", "https://example.com/")
	assert.Equal(t, "https://go.dev/", theStorage.Storage["foo"])
	assert.Equal(t, "https://example.com/", theStorage.Storage["bar"])
}

func TestGetURLFromStorage(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		isErr  bool
		result string
	}{
		{
			name:   "positive case",
			key:    "foo",
			isErr:  false,
			result: "https://practicum.yandex.ru/",
		},
		{
			name:   "unknown key",
			key:    "bar",
			isErr:  true,
			result: "",
		},
		{
			name:   "empty key",
			key:    "",
			isErr:  true,
			result: "",
		},
	}
	theStorage := storage.NewLocmemURLStorerBackend()
	theStorage.Set("foo", "https://practicum.yandex.ru/")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			longURL, err := theStorage.Get(tt.key)
			if tt.isErr {
				assert.Error(t, err)
				assert.Equal(t, "", longURL)
			} else {
				assert.Equal(t, tt.result, longURL)
				assert.NoError(t, err)
			}
		})
	}
}
