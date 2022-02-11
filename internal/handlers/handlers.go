package handlers

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/middleware"
	"github.com/sergeii/practikum-go-url-shortener/storage"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	if longURL == "" {
		return nil, errors.New("please provide a url to shorten")
	}
	if user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser); ok {
		userID = user.ID
	}
	// Получаем короткий идентификатор для ссылки и кладем пару в хранилище
	shortID := handler.App.Hasher.Hash(longURL)
	handler.App.Storage.Set(shortID, longURL, userID)
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
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shortURL, err := handler.shortenAndSaveLongURL(string(body), r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL.String()))
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
	longURL, err := handler.App.Storage.Get(shortURLID)
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

	shortURL, err := handler.shortenAndSaveLongURL(shortenReq.URL, r)
	// Значение параметра невалидно
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respBody, err := json.Marshal(&APIShortenResult{Result: shortURL.String()})
	// Не удалось серилизовать json по некой очень редкой проблеме
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(respBody)
}

func (handler Handler) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser)
	if !ok {
		http.Error(w, "not authenticated", http.StatusForbidden)
		return
	}
	items := handler.App.Storage.GetURLsByUserID(user.ID)
	// Не найдено ни одной ссылки для текущего пользователя
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
	result, err := json.Marshal(jsonItems)
	// Не удалось серилизовать json по некой очень редкой проблеме
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(result)
}
