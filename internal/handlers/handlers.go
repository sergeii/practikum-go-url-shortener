package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/sergeii/practikum-go-url-shortener/pkg/http/resp"

	"github.com/go-chi/chi/v5"
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/middleware"
	"github.com/sergeii/practikum-go-url-shortener/storage"
)

type Handler struct {
	App *app.App
}

func (handler Handler) constructShortURL(shortID string, r *http.Request) *url.URL {
	// Мы возвращаем короткую ссылку используя настройки базового URL сервиса
	// В случае его отстуствия используем имя хоста, с которым был совершен запрос
	baseURLScheme, baseURLHost, baseURLPath := "http", r.Host, "/"
	if handler.App.Config.BaseURL != nil {
		if handler.App.Config.BaseURL.Scheme != "" {
			baseURLScheme = handler.App.Config.BaseURL.Scheme
		}
		if handler.App.Config.BaseURL.Host != "" {
			baseURLHost = handler.App.Config.BaseURL.Host
		}
		if handler.App.Config.BaseURL.Path != "" {
			baseURLPath = handler.App.Config.BaseURL.Path
		}
	}
	shortURLPath := strings.TrimRight(baseURLPath, "/") + "/" + shortID
	return &url.URL{
		Scheme: baseURLScheme,
		Host:   baseURLHost,
		Path:   shortURLPath,
	}
}

func (handler Handler) shortenAndSaveLongURL(longURL string, r *http.Request) (*url.URL, error) {
	var userID string
	if user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser); ok {
		userID = user.ID
	}
	// Получаем короткий идентификатор для ссылки и кладем пару в хранилище
	shortID := handler.App.Hasher.Hash(longURL)
	if err := handler.App.Storage.Set(r.Context(), shortID, longURL, userID); err != nil {
		return nil, err
	}
	shortURL := handler.constructShortURL(shortID, r)
	return shortURL, nil
}

// ShortenURL принимает на вход произвольный URL в теле запроса и создает для него "короткую" версию,
// при переходе по которой пользователь попадет на оригинальный "длинный" URL
// В случае успеха возвращает код 201 и готовую короткую ссылку в теле ответа
// В случае отстуствия валидного URL в теле запроса вернет ошибку 400
func (handler Handler) ShortenURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	// Пытаемся получить длинный url из тела запроса
	longURL := string(body)
	if longURL == "" {
		http.Error(w, "please provide a url to shorten", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shortURL, err := handler.shortenAndSaveLongURL(longURL, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL.String())) // nolint:errcheck
}

// ExpandURL перенаправляет пользователя, перешедшего по короткой ссылке, на оригинальный "длинный" URL.
// В случае успеха возвращает код 307 с редиректом на оригинальный URL
// В случае неизвестной сервису короткой ссылки возвращает ошибку 404
func (handler Handler) ExpandURL(w http.ResponseWriter, r *http.Request) {
	// Пытаемся получить id короткой ссылки из пути
	// и найти по нему длинную ссылку, которую затем возвращаем в виде 307 редиректа
	shortURLID := chi.URLParam(r, "slug")
	if shortURLID == "" {
		http.Error(w, "Invalid short url", http.StatusBadRequest)
		return
	}
	longURL, err := handler.App.Storage.Get(r.Context(), shortURLID)
	if err != nil {
		if errors.Is(err, storage.ErrURLNotFound) {
			// Короткая ссылка не найдена в хранилище - ожидаемое поведение, возвращаем 404
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			// Другая проблема с хранилищем
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, longURL, http.StatusTemporaryRedirect)
}

// APIShortenURL по аналогии с ShortenURL принимает на вход произвольный URL и создает для него короткую ссылку.
// Эндпоинт принимает ссылку в виде json, URL в котором указывается ключем "url"
// В случае успеха возвращает код 201 и готовую короткую ссылку в теле ответа, так же в виде json.
// В случае отстуствия валидного URL в теле запроса вернет ошибку 400
func (handler Handler) APIShortenURL(w http.ResponseWriter, r *http.Request) {
	var shortenReq APIShortenRequest
	// Получили невалидный json
	if err := json.NewDecoder(r.Body).Decode(&shortenReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Значение параметра невалидно
	if shortenReq.URL == "" {
		http.Error(w, "please provide a url to shorten", http.StatusBadRequest)
		return
	}

	shortURL, err := handler.shortenAndSaveLongURL(shortenReq.URL, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resultItem := APIShortenResult{Result: shortURL.String()}
	resp.JSONResponse(&resultItem, w, http.StatusCreated)
}

// GetUserURLs возвращает полный список всех ссылок, сокращенных текущим пользователем.
// Ссылки возвращаются парами Длинный URL + Короткий URL
// В случае отсутствия ссылок у пользователя, возвращается статус 204 без тела ответа
func (handler Handler) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser)
	if !ok {
		http.Error(w, "not authenticated", http.StatusForbidden)
		return
	}
	items, err := handler.App.Storage.GetURLsByUserID(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Не найдено ни одной ссылки для текущего пользователя - возвращаем 204
	if len(items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Серилизуем полученный результат
	jsonItems := make([]APIUserURLItem, 0, len(items))
	for shortID, longURL := range items {
		item := APIUserURLItem{
			ShortURL:    handler.constructShortURL(shortID, r).String(),
			OriginalURL: longURL,
		}
		jsonItems = append(jsonItems, item)
	}
	resp.JSONResponse(&jsonItems, w, http.StatusOK)
}

// Ping проверяет соединение с базой данных и возвращает 200 OK в случае успешной проверки
// В случае наличия проблем с подключением или ошибкой, связанной с превышением времени ожидания ответа,
// возвращает ошибку 500
func (handler Handler) Ping(w http.ResponseWriter, r *http.Request) {
	if handler.App.DB == nil {
		http.Error(w, "database is not available", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), handler.App.Config.DatabaseConnectTimeout)
	defer cancel()
	if err := handler.App.DB.Ping(ctx); err != nil {
		log.Printf("failed to ping database because of %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK")) // nolint:errcheck
}

func (handler Handler) APIShortenBatch(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser)
	if !ok {
		http.Error(w, "not authenticated", http.StatusForbidden)
		return
	}
	shortenBatchReq := make([]APIShortenBatchRequestItem, 0)
	if err := json.NewDecoder(r.Body).Decode(&shortenBatchReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Собираем список для массовой вставки и заодно собираем последущий ответ для клиента
	batchItems := make([]storage.BatchItem, 0, len(shortenBatchReq))
	shortenBatchRes := make([]APIShortenBatchResultItem, 0, len(shortenBatchReq))
	for _, reqItem := range shortenBatchReq {
		if reqItem.OriginalURL == "" {
			continue
		}
		shortID := handler.App.Hasher.Hash(reqItem.OriginalURL)
		batchItem := storage.BatchItem{ShortID: shortID, LongURL: reqItem.OriginalURL, UserID: user.ID}
		resultItem := APIShortenBatchResultItem{
			CorrelationID: reqItem.CorrelationID,
			ShortURL:      handler.constructShortURL(shortID, r).String(),
		}
		shortenBatchRes = append(shortenBatchRes, resultItem)
		batchItems = append(batchItems, batchItem)
	}
	// Проверяем список на пустоту здесь, поскольку некоторые урлы могли быть отсеяны при валидации
	if len(batchItems) == 0 {
		http.Error(w, "please provide a list of urls to shorten", http.StatusBadRequest)
		return
	}
	if err := handler.App.Storage.SaveBatch(r.Context(), batchItems); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp.JSONResponse(&shortenBatchRes, w, http.StatusOK)
}
