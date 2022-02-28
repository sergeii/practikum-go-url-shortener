package storage_test

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/sergeii/practikum-go-url-shortener/storage"
	"github.com/stretchr/testify/assert"
)

func getTestFileStorage() (*storage.FileURLStorerBackend, func()) {
	f, _ := os.CreateTemp("", "*")
	f.Close()
	theStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	return theStorage, func() {
		os.Remove(f.Name())
		theStorage.Close()
	}
}

func TestSetGetFromFileStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()

	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", "") // nolint: errcheck
	URL, _ := theStorage.Get(ctx, "foo")
	assert.Equal(t, "https://practicum.yandex.ru/", URL)
	// Можем перезаписать
	theStorage.Set(ctx, "foo", "https://go.dev/", "") // nolint: errcheck
	URL, _ = theStorage.Get(ctx, "foo")
	assert.Equal(t, "https://go.dev/", URL)

	// Или записать с другим id
	theStorage.Set(ctx, "bar", "https://example.com/", "") // nolint: errcheck
	URL1, _ := theStorage.Get(ctx, "foo")
	URL2, _ := theStorage.Get(ctx, "bar")
	assert.Equal(t, "https://go.dev/", URL1)
	assert.Equal(t, "https://example.com/", URL2)
}

func TestGetURLFromFileStorage(t *testing.T) {
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
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()
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

func TestSaveURLToFileStorageConflict(t *testing.T) {
	ctx := context.TODO()
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()

	shortID, err := theStorage.Set(ctx, "foo", "https://go.dev/", "user1")
	assert.Equal(t, "foo", shortID)
	assert.Nil(t, err)
	url, err := theStorage.Get(ctx, "foo")
	assert.Equal(t, "https://go.dev/", url)
	assert.NoError(t, err)

	for _, userID := range []string{"user1", "user2"} {
		shortID, err = theStorage.Set(ctx, "bar", "https://go.dev/", userID)
		assert.Equal(t, "foo", shortID)
		assert.ErrorIs(t, storage.ErrURLAlreadyExists, err)
		url, err = theStorage.Get(ctx, "bar")
		assert.ErrorIs(t, err, storage.ErrURLNotFound)
		assert.Equal(t, "", url)
	}
}

func TestGetUserURLsFromFileStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()

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

func TestFileStorageIsPersistent(t *testing.T) {
	ctx := context.TODO()
	f, _ := os.CreateTemp("", "*")
	f.Close()
	defer os.Remove(f.Name())

	firstStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	firstStorage.Set(ctx, "foo", "https://go.dev", "") // nolint: errcheck
	firstStorage.Close()

	secondStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	URL, _ := secondStorage.Get(ctx, "foo")
	assert.Equal(t, "https://go.dev", URL)
	secondStorage.Set(ctx, "bar", "https://blog.golang.org/", "user1") // nolint: errcheck
	secondStorage.Close()

	thirdStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	URL1, _ := thirdStorage.Get(ctx, "foo")
	URL2, _ := thirdStorage.Get(ctx, "bar")
	assert.Equal(t, "https://go.dev", URL1)
	assert.Equal(t, "https://blog.golang.org/", URL2)
	thirdStorage.Close()

	savedItems := make(map[string]map[string]string)
	f, _ = os.Open(f.Name())
	defer f.Close()
	json.NewDecoder(f).Decode(&savedItems) // nolint:errcheck
	assert.Equal(t, "https://go.dev", savedItems["foo"]["LongURL"])
	assert.Equal(t, "", savedItems["foo"]["UserID"])
	assert.Equal(t, "https://blog.golang.org/", savedItems["bar"]["LongURL"])
	assert.Equal(t, "user1", savedItems["bar"]["UserID"])
}

func TestFileStorageIsAbleToStartWithoutFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "*")
	filename := path.Join(tmpDir, "saved.json")
	defer os.RemoveAll(tmpDir)

	theStorage, err := storage.NewFileURLStorerBackend(filename)
	assert.NoError(t, err)
	theStorage.Set(context.TODO(), "foo", "https://go.dev/", "") // nolint: errcheck
	theStorage.Close()

	f, _ := os.Open(filename)
	defer f.Close()
	savedItems := make(map[string]map[string]interface{})
	err = json.NewDecoder(f).Decode(&savedItems)
	assert.NoError(t, err)
	assert.Equal(t, "https://go.dev/", savedItems["foo"]["LongURL"])
}

func TestFileStorageIsAbleToStartWithEmptyFile(t *testing.T) {
	f, _ := os.CreateTemp("", "*")
	f.Close()
	defer os.Remove(f.Name())

	theStorage, err := storage.NewFileURLStorerBackend(f.Name())
	assert.NoError(t, err)
	theStorage.Set(context.TODO(), "foo", "https://go.dev/", "") // nolint: errcheck
	theStorage.Close()

	f, _ = os.Open(f.Name())
	defer f.Close()
	savedItems := make(map[string]map[string]interface{})
	err = json.NewDecoder(f).Decode(&savedItems)
	assert.NoError(t, err)
	assert.Equal(t, "https://go.dev/", savedItems["foo"]["LongURL"])
}

func TestFileStorageWontStartWithBrokenJSON(t *testing.T) {
	f, _ := os.CreateTemp("", "*")
	f.Write([]byte(`{foo: "bar"}`)) // nolint:errcheck
	f.Close()
	defer os.Remove(f.Name())

	theStorage, err := storage.NewFileURLStorerBackend(f.Name())
	assert.Nil(t, theStorage)
	assert.IsType(t, err, &json.SyntaxError{})
}

func TestFileStorageDoesNotEscapeHTMLChars(t *testing.T) {
	ctx := context.TODO()
	f, _ := os.CreateTemp("", "*")
	f.Close()
	defer os.Remove(f.Name())

	theStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	theStorage.Set(ctx, "foo", "https://yandex.ru/search/?lr=213&text=golang", "") // nolint: errcheck
	theStorage.Close()

	newStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	URL, _ := newStorage.Get(ctx, "foo")
	assert.Equal(t, "https://yandex.ru/search/?lr=213&text=golang", URL)
	newStorage.Close()

	savedItems := make(map[string]map[string]string)
	f, _ = os.Open(f.Name())
	defer f.Close()
	json.NewDecoder(f).Decode(&savedItems) // nolint:errcheck
	assert.Equal(t, "https://yandex.ru/search/?lr=213&text=golang", savedItems["foo"]["LongURL"])
}

func TestBatchSaveURLsToFileStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()
	theStorage.Set(ctx, "wiki", "https://wikipedia.org/", "u2") // nolint: errcheck

	batchItems := []storage.BatchItem{
		{"ya", "https://ya.ru", "u1"},
		{"go", "https://go.dev/", "u1"},
		{"foo", "https://example.com/", "u1"},
		{"bar", "https://practicum.yandex.ru/", "u1"},
		{"ham", "https://practicum.yandex.ru/", "u1"}, // дубль длинного URL с предыдущей строки
		{"new", "https://wikipedia.org/", "u1"},       // дубль длинного URL существующей записи в бд
	}
	result, err := theStorage.SaveBatch(ctx, batchItems)
	assert.NoError(t, err)
	assert.Len(t, result, 5)
	assert.Equal(t, result["https://wikipedia.org/"], "wiki")
	assert.Equal(t, result["https://practicum.yandex.ru/"], "bar")
	assert.Equal(t, result["https://example.com/"], "foo")
	assert.Equal(t, result["https://go.dev/"], "go")
	assert.Equal(t, result["https://ya.ru"], "ya")
	assert.NotContains(t, result, "https://golang.org/")
}

func TestBatchDeleteUserURLsFromFileStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()

	theStorage.Set(ctx, "wiki", "https://wikipedia.org/", "u1")      // nolint: errcheck
	theStorage.Set(ctx, "go", "https://go.dev/", "u1")               // nolint: errcheck
	theStorage.Set(ctx, "foo", "https://example.com/", "u2")         // nolint: errcheck
	theStorage.Set(ctx, "ya", "https://ya.ru", "u3")                 // nolint: errcheck
	theStorage.Set(ctx, "bar", "https://practicum.yandex.ru/", "u1") // nolint: errcheck

	err := theStorage.DeleteUserURLs(ctx, "u1", "wiki", "go", "foo", "ya", "bar", "unknown")
	assert.NoError(t, err)

	u1Items, _ := theStorage.GetURLsByUserID(ctx, "u1")
	assert.Len(t, u1Items, 0)
	_, err = theStorage.Get(ctx, "go")
	assert.ErrorIs(t, storage.ErrURLIsDeleted, err)

	u2Items, _ := theStorage.GetURLsByUserID(ctx, "u2")
	assert.Len(t, u2Items, 1)
	url, err := theStorage.Get(ctx, "foo")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/", url)

	u3Items, _ := theStorage.GetURLsByUserID(ctx, "u3")
	assert.Len(t, u3Items, 1)

	// можно снова сохранить ссылку без конфликтов
	shortID, err := theStorage.Set(ctx, "wikinew", "https://wikipedia.org/", "u1")
	assert.NoError(t, err)
	assert.Equal(t, "wikinew", shortID)
	// и под другим пользователем
	shortID, err = theStorage.Set(ctx, "gonew", "https://go.dev/", "u2")
	assert.NoError(t, err)
	assert.Equal(t, "gonew", shortID)
}

func TestFileStoragePing(t *testing.T) {
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()
	assert.Nil(t, theStorage.Ping(context.TODO()))
}
