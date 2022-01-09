package main

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
)

type UrlShortenerHandler struct {
	storage storage.UrlStorer
	hasher  hasher.UrlHasher
}

func (handler UrlShortenerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (handler UrlShortenerHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	// Пытаемся получить длинный url из тела запроса
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	longUrl := string(body)
	if longUrl == "" {
		http.Error(w, "Please provide a url to shorten", 400)
		return
	}
	// Получаем короткий идентификатор для ссылки и кладем пару в хранилище
	shortUrlId := handler.hasher.HashUrl(longUrl)
	handler.storage.Set(shortUrlId, longUrl)
	// Возвращаем короткую ссылку с учетом хоста, на котором запущен сервис
	shortUrl := "http://" + r.Host + "/" + shortUrlId
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortUrl))
}

func (handler UrlShortenerHandler) Expand(w http.ResponseWriter, r *http.Request) {
	// Пытаемся получить id короткой ссылки из пути
	// и найти по нему длинную ссылку, которую затем возвращаем в виде 307 редиректа
	shortUrlId := strings.Trim(r.URL.Path, "/")
	if shortUrlId == "" {
		http.Error(w, "Invalid short url", 400)
		return
	}
	longUrl, err := handler.storage.Get(shortUrlId)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, longUrl, http.StatusTemporaryRedirect)
}

func main() {
	server := &http.Server{
		Addr: "localhost:8080",
		Handler: &UrlShortenerHandler{
			storage: storage.NewLocmemUrlShortenerBackend(),
			hasher:  &hasher.SimpleUrlHasher{},
		},
	}
	log.Fatal(server.ListenAndServe())
}
