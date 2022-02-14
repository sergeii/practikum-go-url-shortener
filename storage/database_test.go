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
	}
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", "user1") // nolint: errcheck

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

func TestGetUserURLsFromDatabaseStorage(t *testing.T) {
	ctx := context.TODO()
	theStorage := getDatabaseStorage(t)
	user1, user2 := "user1", "user2" // nolint: goconst

	theStorage.Set(ctx, "foo", "https://practicum.yandex.ru/", user1) // nolint: errcheck
	theStorage.Set(ctx, "bar", "https://go.dev/", user1)              // nolint: errcheck
	theStorage.Set(ctx, "foo", "https://google.com/", user2)          // nolint: errcheck
	theStorage.Set(ctx, "baz", "https://exampe.com/", user2)          // nolint: errcheck
	theStorage.Set(ctx, "ham", "https://wikipedia.org/", "")          // nolint: errcheck

	user1Items, _ := theStorage.GetURLsByUserID(ctx, user1)
	assert.Len(t, user1Items, 2)
	assert.Contains(t, user1Items, "foo")
	assert.Contains(t, user1Items, "bar")

	user2Items, _ := theStorage.GetURLsByUserID(ctx, user2)
	assert.Len(t, user2Items, 2)
	assert.Contains(t, user2Items, "foo")
	assert.Contains(t, user2Items, "baz")

	emptyUserItems, _ := theStorage.GetURLsByUserID(ctx, "")
	assert.Len(t, emptyUserItems, 0)
}
