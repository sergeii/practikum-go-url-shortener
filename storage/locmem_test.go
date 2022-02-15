package storage_test

import (
	"context"
	"testing"

	"github.com/sergeii/practikum-go-url-shortener/storage"
	"github.com/stretchr/testify/assert"
)

func TestSaveURLToLocmemStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage := storage.NewLocmemURLStorerBackend()

	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", "") // nolint: errcheck
	assert.Equal(t, "https://practicum.yandex.ru/", theStorage.Storage["foo"].LongURL)
	// Можем перезаписать
	theStorage.Set(ctx, "foo", "https://go.dev/", "") // nolint: errcheck
	assert.Equal(t, "https://go.dev/", theStorage.Storage["foo"].LongURL)

	// Или записать с другим id
	theStorage.Set(ctx, "bar", "https://example.com/", "user1") // nolint: errcheck
	assert.Equal(t, "https://go.dev/", theStorage.Storage["foo"].LongURL)
	assert.Equal(t, "", theStorage.Storage["foo"].UserID)
	assert.Equal(t, "https://example.com/", theStorage.Storage["bar"].LongURL)
	assert.Equal(t, "user1", theStorage.Storage["bar"].UserID)
}

func TestGetURLFromLocmemStorage(t *testing.T) {
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
	ctx := context.TODO()
	theStorage := storage.NewLocmemURLStorerBackend()
	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", "") // nolint: errcheck

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			longURL, err := theStorage.Get(ctx, tt.key)
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

func TestGetUserURLsFromLocmemStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage := storage.NewLocmemURLStorerBackend()
	user1, user2 := "user1", "user2"

	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", user1) // nolint: errcheck
	theStorage.Set(ctx, "bar", "https://go.dev/", user1)              // nolint: errcheck
	theStorage.Set(ctx, "baz", "https://google.com/", user2)          // nolint: errcheck
	theStorage.Set(ctx, "ham", "https://google.com/", "")             // nolint: errcheck

	user1Items, _ := theStorage.GetURLsByUserID(ctx, user1)
	assert.Len(t, user1Items, 2)
	assert.Contains(t, user1Items, "foo")
	assert.Contains(t, user1Items, "bar")

	user2Items, _ := theStorage.GetURLsByUserID(ctx, user2)
	assert.Len(t, user2Items, 1)
	assert.Contains(t, user2Items, "baz")

	emptyUserItems, _ := theStorage.GetURLsByUserID(ctx, "")
	assert.Len(t, emptyUserItems, 0)
}
