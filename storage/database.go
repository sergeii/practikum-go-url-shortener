package storage

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/lib/pq"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DatabaseURLStorerBackend struct {
	DB      *pgxpool.Pool
	timeout time.Duration
}

type conn interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

const initDatabaseSQL = `
CREATE TABLE IF NOT EXISTS urls (
    id SERIAL PRIMARY KEY,
    short_id TEXT,
    original_url TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    is_deleted boolean NOT NULL DEFAULT FALSE,
    CHECK (user_id <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS urls_short_id_uniq_idx ON urls (short_id);
ALTER TABLE urls DROP CONSTRAINT IF EXISTS urls_short_id_uniq;
ALTER TABLE urls ADD CONSTRAINT urls_short_id_uniq UNIQUE USING INDEX urls_short_id_uniq_idx;

CREATE UNIQUE INDEX IF NOT EXISTS urls_original_url_uniq_idx ON urls (original_url) WHERE is_deleted = false;

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
			"ON CONFLICT (original_url) WHERE is_deleted = false DO NOTHING RETURNING id",
		shortURLID, longURL, userID,
	).Scan(&rowID)
	if err != nil {
		// при попытке сохранить уже сокращенный URL (и при этом не удаленный)
		// получим конфликт, и строка не вставится, вернув нам ничего
		// что мы и обрабатываем, доставая из бд ранее сохраненную ссылку, о которую столкнулся запрос
		if errors.Is(err, pgx.ErrNoRows) {
			actualShortID, recoveryErr := backend.getShortIDForURL(ctx, backend.DB, longURL)
			if recoveryErr != nil {
				return "", recoveryErr
			}
			return actualShortID, ErrURLAlreadyExists
		}
		return "", err
	}
	return shortURLID, nil
}

func (backend DatabaseURLStorerBackend) Get(ctx context.Context, shortURLID string) (string, error) {
	var longURL string
	var isDeleted bool

	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()

	row := backend.DB.QueryRow(ctx, "SELECT original_url, is_deleted FROM urls WHERE short_id = $1", shortURLID)
	if err := row.Scan(&longURL, &isDeleted); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrURLNotFound
		}
		return "", err
	}
	if isDeleted {
		return "", ErrURLIsDeleted
	}
	return longURL, nil
}

func (backend DatabaseURLStorerBackend) GetURLsByUserID(ctx context.Context, userID string) (map[string]string, error) {
	var shortURL, longURL string

	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()

	rows, err := backend.DB.Query(
		ctx, "SELECT short_id, original_url FROM urls WHERE user_id = $1 AND is_deleted = false", userID,
	)
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

func (backend DatabaseURLStorerBackend) DeleteUserURLs(ctx context.Context, userID string, shortIDs ...string) error {
	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()

	sql := "UPDATE urls SET is_deleted = true WHERE user_id = $1 AND short_id = ANY($2)"
	result, err := backend.DB.Exec(ctx, sql, userID, pq.Array(shortIDs))
	if err != nil {
		return err
	}

	log.Printf("deleted %d urls for user %s", result.RowsAffected(), userID)
	return nil
}

func (backend DatabaseURLStorerBackend) SaveBatch(ctx context.Context, items []BatchItem) (map[string]string, error) {
	var rowID int

	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()

	tx, err := backend.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func(ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			log.Printf("failed to rollback transaction due to %v", err)
		}
	}(ctx)

	prepSQL := "INSERT INTO urls (short_id, original_url, user_id) VALUES($1,$2,$3) " +
		"ON CONFLICT (original_url) WHERE is_deleted = false DO NOTHING RETURNING id"
	if _, err := tx.Prepare(ctx, "batch-insert", prepSQL); err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, item := range items {
		err = tx.QueryRow(ctx, "batch-insert", item.ShortID, item.LongURL, item.UserID).Scan(&rowID)
		if err != nil {
			// строка не записалась из-за конфликта - пытаемся получить идентификатор ранее сокращенной ссылки
			if errors.Is(err, pgx.ErrNoRows) {
				actualShortID, recoveryErr := backend.getShortIDForURL(ctx, tx, item.LongURL)
				// не получилось разрешить конфликт - пропускаем ссылку
				if recoveryErr != nil {
					return nil, recoveryErr
				}
				result[item.LongURL] = actualShortID
			} else {
				return nil, err
			}
		} else {
			result[item.LongURL] = item.ShortID
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
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

func (backend DatabaseURLStorerBackend) getShortIDForURL(ctx context.Context, conn conn, url string) (string, error) {
	var shortID string
	ctx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()
	row := conn.QueryRow(ctx, "SELECT short_id FROM urls WHERE original_url = $1 AND is_deleted = false", url)
	if err := row.Scan(&shortID); err != nil {
		return "", err
	}
	return shortID, nil
}
