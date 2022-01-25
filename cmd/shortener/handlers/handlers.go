package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

type URLShortenerHandler struct {
	Storage storage.URLStorer
	Hasher  hasher.URLHasher
	BaseURL *url.URL
}

func (handler URLShortenerHandler) makeShortURL(longURL string, r *http.Request) (*url.URL, error) {
	if longURL == "" {
		return nil, errors.New("please provide a url to shorten")
	}
	// Получаем короткий идентификатор для ссылки и кладем пару в хранилище
	shortURLID := handler.Hasher.HashURL(longURL)
	handler.Storage.Set(shortURLID, longURL)
	// Мы возвращаем короткую ссылку используя настройки базового URL сервиса
	// В случае его отстуствия используем имя хоста, с которым был совершен запрос
	baseURLScheme, baseURLHost, baseURLPath := "http", r.Host, "/"
	if handler.BaseURL != nil {
		if handler.BaseURL.Scheme != "" {
			baseURLScheme = handler.BaseURL.Scheme
		}
		if handler.BaseURL.Host != "" {
			baseURLHost = handler.BaseURL.Host
		}
		if handler.BaseURL.Path != "" {
			baseURLPath = handler.BaseURL.Path
		}
	}
	shortURLPath := strings.TrimRight(baseURLPath, "/") + "/" + shortURLID
	return &url.URL{
		Scheme: baseURLScheme,
		Host:   baseURLHost,
		Path:   shortURLPath,
	}, nil
}

// ShortenURL принимает на вход произвольный URL в теле запроса и создает для него "короткую" версию,
// при переходе по которой пользователь попадет на оригинальный "длинный" URL
// В случае успеха возвращает код 201 и готовую короткую ссылку в теле ответа
// В случае отстуствия валидного URL в теле запроса вернет ошибку 400
func (handler URLShortenerHandler) ShortenURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	// Пытаемся получить длинный url из тела запроса
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shortURL, err := handler.makeShortURL(string(body), r)
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
func (handler URLShortenerHandler) ExpandURL(w http.ResponseWriter, r *http.Request) {
	// Пытаемся получить id короткой ссылки из пути
	// и найти по нему длинную ссылку, которую затем возвращаем в виде 307 редиректа
	shortURLID := chi.URLParam(r, "slug")
	if shortURLID == "" {
		http.Error(w, "Invalid short url", http.StatusBadRequest)
		return
	}
	longURL, err := handler.Storage.Get(shortURLID)
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

type APIShortenRequest struct {
	URL string `json:"url"` // Оригинальный длинный URL, требующий укорачивания
}

type APIShortenResult struct {
	Result string `json:"result"` // Короткий URL, превращенный из длинного
}

// APIShortenURL по аналогии с ShortenURL принимает на вход произвольный URL и создает для него короткую ссылку.
// Эндпоинт принимает ссылку в виде json, URL в котором указывается ключем "url"
// В случае успеха возвращает код 201 и готовую короткую ссылку в теле ответа, так же в виде json.
// В случае отстуствия валидного URL в теле запроса вернет ошибку 400
func (handler URLShortenerHandler) APIShortenURL(w http.ResponseWriter, r *http.Request) {
	var shortenReq APIShortenRequest
	// Получили невалидный json
	if err := json.NewDecoder(r.Body).Decode(&shortenReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	shortURL, err := handler.makeShortURL(shortenReq.URL, r)
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
