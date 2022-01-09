package main

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

type URLShortenerHandler struct {
	storage storage.URLStorer
	hasher  hasher.URLHasher
}

func (handler URLShortenerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	// Пытаемся принять длинный url и превратить его в короткий
	case "POST":
		handler.Shorten(w, r)
	// Пытаемся принять короткий url и вернуть оригинальный длинный
	case "GET":
		handler.Expand(w, r)
	default:
		http.Error(w, "Invalid request method", 400)
	}
}

func (handler URLShortenerHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
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
	shortURLID := handler.hasher.HashURL(longURL)
	handler.storage.Set(shortURLID, longURL)
	// Возвращаем короткую ссылку с учетом хоста, на котором запущен сервис
	shortURL := "http://" + r.Host + "/" + shortURLID
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (handler URLShortenerHandler) Expand(w http.ResponseWriter, r *http.Request) {
	// Пытаемся получить id короткой ссылки из пути
	// и найти по нему длинную ссылку, которую затем возвращаем в виде 307 редиректа
	shortURLID := strings.Trim(r.URL.Path, "/")
	if shortURLID == "" {
		http.Error(w, "Invalid short url", 400)
		return
	}
	longURL, err := handler.storage.Get(shortURLID)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, longURL, http.StatusTemporaryRedirect)
}

func main() {
	server := &http.Server{
		Addr: "localhost:8080",
		Handler: &URLShortenerHandler{
			storage: storage.NewLocmemURLShortenerBackend(),
			hasher:  &hasher.SimpleURLHasher{},
		},
	}
	log.Fatal(server.ListenAndServe())
}
