package handlers

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

type URLShortenerHandler struct {
	Storage storage.URLStorer
	Hasher  hasher.URLHasher
}

func (handler URLShortenerHandler) ShortenURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	// Пытаемся получить длинный url из тела запроса
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	longURL := string(body)
	if longURL == "" {
		http.Error(w, "Please provide a url to shorten", 400)
		return
	}
	// Получаем короткий идентификатор для ссылки и кладем пару в хранилище
	shortURLID := handler.Hasher.HashURL(longURL)
	handler.Storage.Set(shortURLID, longURL)
	// Возвращаем короткую ссылку с учетом хоста, на котором запущен сервис
	shortURL := "http://" + r.Host + "/" + shortURLID
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (handler URLShortenerHandler) ExpandURL(w http.ResponseWriter, r *http.Request) {
	// Пытаемся получить id короткой ссылки из пути
	// и найти по нему длинную ссылку, которую затем возвращаем в виде 307 редиректа
	shortURLID := chi.URLParam(r, "slug")
	if shortURLID == "" {
		http.Error(w, "Invalid short url", 400)
		return
	}
	longURL, err := handler.Storage.Get(shortURLID)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, longURL, http.StatusTemporaryRedirect)
}
