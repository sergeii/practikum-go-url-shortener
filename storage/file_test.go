package storage_test

import (
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
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()

	theStorage.Set("foo", "https://practicum.yandex.ru/", "")
	URL, _ := theStorage.Get("foo")
	assert.Equal(t, "https://practicum.yandex.ru/", URL)
	// Можем перезаписать
	theStorage.Set("foo", "https://go.dev/", "")
	URL, _ = theStorage.Get("foo")
	assert.Equal(t, "https://go.dev/", URL)

	// Или записать с другим id
	theStorage.Set("bar", "https://example.com/", "")
	URL1, _ := theStorage.Get("foo")
	URL2, _ := theStorage.Get("bar")
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

	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theStorage.Set("foo", "https://practicum.yandex.ru/", "")
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

func TestGetUserURLsFromFileStorage(t *testing.T) {
	theStorage, closeFunc := getTestFileStorage()
	defer closeFunc()

	user1, user2 := "user1", "user2"

	theStorage.Set("foo", "https://practicum.yandex.ru/", user1)
	theStorage.Set("bar", "https://go.dev/", user1)
	theStorage.Set("baz", "https://google.com/", user2)
	theStorage.Set("ham", "https://google.com/", "")

	user1Items := theStorage.GetURLsByUserID(user1)
	assert.Len(t, user1Items, 2)
	assert.Contains(t, user1Items, "foo")
	assert.Contains(t, user1Items, "bar")

	user2Items := theStorage.GetURLsByUserID(user2)
	assert.Len(t, user2Items, 1)
	assert.Contains(t, user2Items, "baz")

	emptyUserItems := theStorage.GetURLsByUserID("")
	assert.Len(t, emptyUserItems, 0)
}

func TestFileStorageIsPersistent(t *testing.T) {
	f, _ := os.CreateTemp("", "*")
	f.Close()
	defer os.Remove(f.Name())

	firstStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	firstStorage.Set("foo", "https://go.dev", "")
	firstStorage.Close()

	secondStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	URL, _ := secondStorage.Get("foo")
	assert.Equal(t, "https://go.dev", URL)
	secondStorage.Set("bar", "https://blog.golang.org/", "user1")
	secondStorage.Close()

	thirdStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	URL1, _ := thirdStorage.Get("foo")
	URL2, _ := thirdStorage.Get("bar")
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
	theStorage.Set("foo", "https://go.dev/", "")
	theStorage.Close()

	f, _ := os.Open(filename)
	defer f.Close()
	savedItems := make(map[string]map[string]string)
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
	theStorage.Set("foo", "https://go.dev/", "")
	theStorage.Close()

	f, _ = os.Open(f.Name())
	defer f.Close()
	savedItems := make(map[string]map[string]string)
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
	f, _ := os.CreateTemp("", "*")
	f.Close()
	defer os.Remove(f.Name())

	theStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	theStorage.Set("foo", "https://yandex.ru/search/?lr=213&text=golang", "")
	theStorage.Close()

	newStorage, _ := storage.NewFileURLStorerBackend(f.Name())
	URL, _ := newStorage.Get("foo")
	assert.Equal(t, "https://yandex.ru/search/?lr=213&text=golang", URL)
	newStorage.Close()

	savedItems := make(map[string]map[string]string)
	f, _ = os.Open(f.Name())
	defer f.Close()
	json.NewDecoder(f).Decode(&savedItems) // nolint:errcheck
	assert.Equal(t, "https://yandex.ru/search/?lr=213&text=golang", savedItems["foo"]["LongURL"])
}
