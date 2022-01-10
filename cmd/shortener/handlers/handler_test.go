package handlers

import (
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/sergeii/practikum-go-url-shortener/internal/app/storage"
	"github.com/stretchr/testify/assert"
)

func TestShortenAndExpandAnyLengthURLs(t *testing.T) {
	TestURLs := []string{
		"https://ya.ru",
		"https://practicum.yandex.ru/learn/go-developer/",
		"https://www.google.com/search?q=golang&client=safari&ei=k3jbYbeDNMOxrgT-ha3gBA&start=10&sa=N&ved=2ahUKEwj3mPjk8aX1AhXDmIsKHf5CC0wQ8tMDegQIAhA5&biw=1280&bih=630&dpr=2",
	}
	handler := URLShortenerHandler{
		Storage: storage.NewLocmemURLStorerBackend(),
		Hasher:  &hasher.SimpleURLHasher{},
	}
	for _, TestURL := range TestURLs {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(TestURL))
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, request)
		result := writer.Result()

		assert.Equal(t, 201, result.StatusCode)

		shortURLResult, err := ioutil.ReadAll(result.Body)
		require.NoError(t, err)
		err = result.Body.Close()
		require.NoError(t, err)

		// Получаем относительный url, состоящий из 7 символов, и пробуем перейти по нему
		parsed, _ := url.Parse(string(shortURLResult))
		assert.Len(t, strings.Trim(parsed.Path, "/"), 7)
		request = httptest.NewRequest(http.MethodGet, parsed.Path, nil)
		writer = httptest.NewRecorder()

		handler.ServeHTTP(writer, request)
		result = writer.Result()
		result.Body.Close()
		// Получем ожидаем редирект на оригинальный url
		assert.Equal(t, 307, result.StatusCode)
		assert.Equal(t, TestURL, result.Header.Get("Location"))
	}
}

func TestUnsupportedHTTPMethods(t *testing.T) {
	HTTPMethods := []string{http.MethodPut, http.MethodHead, http.MethodDelete}
	handler := URLShortenerHandler{
		Storage: storage.NewLocmemURLStorerBackend(),
		Hasher:  &hasher.SimpleURLHasher{},
	}
	for _, HTTPMethod := range HTTPMethods {
		request := httptest.NewRequest(HTTPMethod, "/", strings.NewReader("https://example.com/"))
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, request)
		result := writer.Result()
		result.Body.Close()
		assert.Equal(t, 400, result.StatusCode)
	}
}

func TestShortenEndpointRequiresURL(t *testing.T) {
	tests := []struct {
		name  string
		body  io.Reader
		isErr bool
	}{
		{
			name:  "positive case",
			body:  strings.NewReader("https://example.com/"),
			isErr: false,
		},
		{
			name:  "empty body",
			body:  strings.NewReader(""),
			isErr: true,
		},
		{
			name:  "no empty provided",
			body:  nil,
			isErr: true,
		},
	}
	handler := URLShortenerHandler{
		Storage: storage.NewLocmemURLStorerBackend(),
		Hasher:  &hasher.SimpleURLHasher{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", tt.body)
			writer := httptest.NewRecorder()
			handler.ServeHTTP(writer, request)
			result := writer.Result()
			result.Body.Close()
			if tt.isErr {
				assert.Equal(t, 400, result.StatusCode)
			} else {
				assert.Equal(t, 201, result.StatusCode)
			}
		})
	}
}

func TestExpandEndpointRequiresProperID(t *testing.T) {
	tests := []struct {
		name   string
		req    string
		isErr  bool
		result string
	}{
		{
			name:   "positive case",
			req:    "/gogogo",
			isErr:  false,
			result: "https://go.dev/",
		},
		{
			name:   "invalid url",
			req:    "/",
			isErr:  true,
			result: "",
		},
		{
			name:   "empty short url id",
			req:    "//",
			isErr:  true,
			result: "",
		},
		{
			name:   "unknown short url",
			req:    "/foobar/",
			isErr:  true,
			result: "",
		},
	}
	handler := URLShortenerHandler{
		Storage: &storage.LocmemURLStorerBackend{
			Storage: map[string]string{
				"gogogo": "https://go.dev/",
			},
		},
		Hasher: &hasher.SimpleURLHasher{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.req, nil)
			writer := httptest.NewRecorder()
			handler.ServeHTTP(writer, request)
			result := writer.Result()
			result.Body.Close()
			if tt.isErr {
				assert.Equal(t, 400, result.StatusCode)
			} else {
				assert.Equal(t, 307, result.StatusCode)
				assert.Equal(t, tt.result, result.Header.Get("Location"))
			}
		})
	}
}

func TestExpandEndpointHandlesExtraSlashes(t *testing.T) {
	TestURLs := []string{
		"/gogogo",
		"/gogogo/",
		"//gogogo/",
		"//gogogo//",
	}
	handler := URLShortenerHandler{
		Storage: &storage.LocmemURLStorerBackend{
			Storage: map[string]string{
				"gogogo": "https://go.dev/",
			},
		},
		Hasher: &hasher.SimpleURLHasher{},
	}
	for _, TestURL := range TestURLs {
		request := httptest.NewRequest(http.MethodGet, TestURL, nil)
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, request)
		result := writer.Result()
		result.Body.Close()
		assert.Equal(t, 307, result.StatusCode)
		assert.Equal(t, "https://go.dev/", result.Header.Get("Location"))
	}
}
