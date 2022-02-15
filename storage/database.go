package storage

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DatabaseURLStorerBackend struct {
	DB      *pgxpool.Pool
	timeout time.Duration
}

const initDatabaseSQL = `
CREATE TABLE IF NOT EXISTS urls (
    id SERIAL PRIMARY KEY,
    short_id TEXT,
    original_url TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CHECK (user_id <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS urls_original_url_uniq_idx ON urls (original_url);
ALTER TABLE urls DROP CONSTRAINT IF EXISTS urls_original_url_uniq;
ALTER TABLE urls ADD CONSTRAINT urls_original_url_uniq UNIQUE USING INDEX urls_original_url_uniq_idx;

CREATE INDEX IF NOT EXISTS urls_user_id_idx ON urls(user_id);
`

func NewDatabaseURLStorerBackend(db *pgxpool.Pool, timeout time.Duration) (*DatabaseURLStorerBackend, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// при инициализации бэкенда создаем по необходимости нужные нам сущности в бд
	if _, err := db.Exec(ctx, initDatabaseSQL); err != nil {
		return nil, err
	}
	return &DatabaseURLStorerBackend{db, timeout}, nil
}

func (backend DatabaseURLStorerBackend) Set(ctx context.Context, shortURLID, longURL, userID string) (string, error) {
	var rowID int
	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()
	err := backend.DB.QueryRow(
		ctx,
		"INSERT INTO urls (short_id, original_url, user_id) VALUES($1, $2, $3) "+
			"ON CONFLICT DO NOTHING RETURNING id",
		shortURLID, longURL, userID,
	).Scan(&rowID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			actualShortID, _ := backend.getShortIDForURL(ctx, longURL)
			return actualShortID, ErrURLAlreadyExists
		}
		return "", err
	}
	return shortURLID, nil
}

func (backend DatabaseURLStorerBackend) getShortIDForURL(ctx context.Context, longURL string) (string, error) {
	var shortID string
	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()
	err := backend.DB.QueryRow(ctx, "SELECT short_id FROM urls WHERE original_url = $1", longURL).Scan(&shortID)
	if err != nil {
		return "", err
	}
	return shortID, nil
}

func (backend DatabaseURLStorerBackend) Get(ctx context.Context, shortURLID string) (string, error) {
	var longURL string

	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()

	err := backend.DB.QueryRow(ctx, "SELECT original_url FROM urls WHERE short_id = $1 LIMIT 1", shortURLID).Scan(&longURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrURLNotFound
		}
		return "", err
	}

	return longURL, nil
}

func (backend DatabaseURLStorerBackend) GetURLsByUserID(ctx context.Context, userID string) (map[string]string, error) {
	var shortURL, longURL string

	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()

	rows, err := backend.DB.Query(ctx, "SELECT short_id, original_url FROM urls WHERE user_id = $1", userID)
	if err != nil {
		log.Printf("failed to query urls of user %s due to %v\n", userID, err)
		return nil, err
	}
	defer rows.Close()

	items := make(map[string]string)
	for rows.Next() {
		err = rows.Scan(&shortURL, &longURL)
		if err != nil {
			log.Printf("failed to read urls of user %s due to %v\n", userID, err)
			return nil, err
		}
		items[shortURL] = longURL
	}
	err = rows.Err()
	if err != nil {
		log.Printf("failed to fetch urls of user %s due to %v\n", userID, err)
		return nil, err
	}
	return items, nil
}

func (backend DatabaseURLStorerBackend) SaveBatch(ctx context.Context, items []BatchItem) error {
	tx, err := backend.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func(ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			log.Printf("failed to rollback transaction due to %v", err)
		}
	}(ctx)

	prepSQL := "INSERT INTO urls (short_id, original_url, user_id) VALUES($1,$2,$3) " +
		"ON CONFLICT ON CONSTRAINT urls_original_url_uniq DO NOTHING"
	if _, err := tx.Prepare(ctx, "batch", prepSQL); err != nil {
		return err
	}

	for _, item := range items {
		if _, err = tx.Exec(ctx, "batch", item.ShortID, item.LongURL, item.UserID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (backend DatabaseURLStorerBackend) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()
	if err := backend.DB.Ping(ctx); err != nil {
		log.Printf("failed to ping database because of %s\n", err)
		return err
	}
	return nil
}

// Cleanup отчищает таблицу с сокращенными урлами с помощью вызова TRUNCATE
// Метод предназначен только для вызовов в тестах
func (backend DatabaseURLStorerBackend) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), backend.timeout)
	defer cancel()
	if _, err := backend.DB.Exec(ctx, "TRUNCATE TABLE urls"); err != nil {
		panic(err)
	}
}

func (backend DatabaseURLStorerBackend) Close() error {
	// соединение к бд закрывается на уровне приложения
	return nil
}
