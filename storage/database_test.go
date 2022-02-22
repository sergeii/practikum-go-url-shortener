package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/storage"
	"github.com/stretchr/testify/assert"
)

func getDatabaseStorage(t *testing.T) *storage.DatabaseURLStorerBackend {
	shorterner, err := app.New()
	if err != nil {
		panic(err)
	}
	if shorterner.DB == nil {
		t.Skip("Skipping test because DB is not configured")
	}
	theStorage, err := storage.NewDatabaseURLStorerBackend(shorterner.DB, shorterner.Config.DatabaseQueryTimeout)
	if err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		theStorage.Cleanup()
		shorterner.Close()
	})
	return theStorage
}

type URLTableRow struct {
	ShortID string
	LongURL string
	UserID  string
}

func getRowForShortID(db *pgxpool.Pool, shortID string) *URLTableRow {
	var row URLTableRow
	err := db.QueryRow(
		context.Background(), "SELECT short_id, original_url, user_id FROM urls WHERE short_id = $1", shortID,
	).Scan(&row.ShortID, &row.LongURL, &row.UserID)
	if err != nil {
		return nil
	}
	return &row
}

func TestSaveURLToDatabaseStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
	db := theStorage.DB

	theStorage.Set(ctx, "foo", "https://go.dev/", "user1") // nolint: errcheck
	assert.Equal(t, "https://go.dev/", getRowForShortID(db, "foo").LongURL)

	// Или записать с другим id
	theStorage.Set(ctx, "bar", "https://example.com/", "user1") // nolint: errcheck
	fooRow := getRowForShortID(db, "foo")
	barRow := getRowForShortID(db, "bar")
	assert.Equal(t, "https://go.dev/", fooRow.LongURL)
	assert.Equal(t, "user1", fooRow.UserID)
	assert.Equal(t, "https://example.com/", barRow.LongURL)
	assert.Equal(t, "user1", barRow.UserID)
}

func TestUnableToSaveURLToDatabaseStorageEmptyUser(t *testing.T) {
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
	db := theStorage.DB

	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", "") // nolint: errcheck
	assert.Nil(t, getRowForShortID(db, "foo"))
	// Можем перезаписать
	theStorage.Set(ctx, "bar", "https://go.dev/", "user1") // nolint: errcheck
	assert.Equal(t, "https://go.dev/", getRowForShortID(db, "bar").LongURL)
}

func TestSaveURLToDatabaseStorageConflict(t *testing.T) {
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
	db := theStorage.DB

	shortID, err := theStorage.Set(ctx, "foo", "https://go.dev/", "user1")
	assert.Equal(t, "foo", shortID)
	assert.Nil(t, err)
	assert.Equal(t, "https://go.dev/", getRowForShortID(db, "foo").LongURL)

	for _, userID := range []string{"user1", "user2"} {
		shortID, err = theStorage.Set(ctx, "bar", "https://go.dev/", userID)
		assert.Equal(t, "foo", shortID)
		assert.ErrorIs(t, storage.ErrURLAlreadyExists, err)
		assert.Nil(t, getRowForShortID(db, "bar"))
	}
}

func TestGetURLFromDatabaseStorage(t *testing.T) {
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
		{
			name:   "deleted url",
			key:    "go",
			isErr:  true,
			result: "",
		},
	}
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", "user1")                // nolint: errcheck
	theStorage.Set(ctx, "go", "https://go.dev/", "user1")                              // nolint: errcheck
	theStorage.DB.Exec(ctx, "UPDATE urls SET is_deleted = true WHERE short_id = 'go'") // nolint: errcheck

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			longURL, err := theStorage.Get(ctx, tt.key)
			if tt.isErr {
				assert.Error(t, err)
				assert.Equal(t, "", longURL, tt.name)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.result, longURL, tt.name)
			}
		})
	}
}

func TestGetUserURLsFromDatabaseStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
	user1, user2 := "user1", "user2" // nolint: goconst

	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", user1) // nolint: errcheck
	theStorage.Set(ctx, "bar", "https://go.dev/", user1)              // nolint: errcheck
	theStorage.Set(ctx, "ham", "https://google.com/", user2)          // nolint: errcheck
	theStorage.Set(ctx, "baz", "https://exampe.com/", user2)          // nolint: errcheck
	theStorage.Set(ctx, "eggs", "https://wikipedia.org/", "")         // nolint: errcheck

	user1Items, _ := theStorage.GetURLsByUserID(ctx, user1)
	assert.Len(t, user1Items, 2)
	assert.Contains(t, user1Items, "foo")
	assert.Contains(t, user1Items, "bar")

	user2Items, _ := theStorage.GetURLsByUserID(ctx, user2)
	assert.Len(t, user2Items, 2)
	assert.Contains(t, user2Items, "ham")
	assert.Contains(t, user2Items, "baz")

	emptyUserItems, _ := theStorage.GetURLsByUserID(ctx, "")
	assert.Len(t, emptyUserItems, 0)

	theStorage.DB.Exec(ctx, "UPDATE urls SET is_deleted = true WHERE short_id = 'foo'") // nolint: errcheck
	updatedUser1Items, _ := theStorage.GetURLsByUserID(ctx, user1)
	assert.Len(t, updatedUser1Items, 1)
	assert.NotContains(t, updatedUser1Items, "foo")
	assert.Contains(t, updatedUser1Items, "bar")
}

func TestSaveURLsToDatabaseBatch(t *testing.T) {
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
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

func TestDeleteUserURLsFromDatabaseBatch(t *testing.T) {
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
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

func TestDatabaseStoragePing(t *testing.T) {
	theStorage := getDatabaseStorage(t)
	assert.Nil(t, theStorage.Ping(context.TODO()))
}
