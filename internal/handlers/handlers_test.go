package handlers_test

import (
	"bytes"
	"encoding/json"
	"github.com/sergeii/practikum-go-url-shortener/config"
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func prepareTestServer(cfg *config.Config) (*httptest.Server, func()) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	shorterner, _ := app.New(cfg)
	ts := httptest.NewServer(router.New(shorterner))
	return ts, func() {
		ts.Close()
		shorterner.Close()
	}
}

func doTestRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	require.NoError(t, err)

	// Отключаем редиректы
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)

	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(respBody)
}

func TestShortenAndExpandAnyLengthURLs(t *testing.T) {
	TestURLs := []string{
		"https://ya.ru",
		"https://practicum.yandex.ru/learn/go-developer/",
		"https://www.google.com/search?q=golang&client=safari&ei=k3jbYbeDNMOxrgT-ha3gBA&start=10&sa=N&ved=2ahUKEwj3mPjk8aX1AhXDmIsKHf5CC0wQ8tMDegQIAhA5&biw=1280&bih=630&dpr=2",
	}

	ts, stop := prepareTestServer(nil)
	defer stop()
	for _, TestURL := range TestURLs {
		resp, body := doTestRequest(t, ts, http.MethodPost, "/", strings.NewReader(TestURL))
		resp.Body.Close()
		assert.Equal(t, 201, resp.StatusCode)

		// Получаем относительный url, состоящий из 7 символов, и пробуем перейти по нему
		parsed, _ := url.Parse(body)
		assert.Len(t, strings.Trim(parsed.Path, "/"), 7)
		resp, _ = doTestRequest(t, ts, http.MethodGet, parsed.Path, nil)
		resp.Body.Close()
		// Получем ожидаем редирект на оригинальный url
		assert.Equal(t, 307, resp.StatusCode)
		assert.Equal(t, TestURL, resp.Header.Get("Location"))
	}
}

func TestShortenEndpointUnsupportedHTTPMethods(t *testing.T) {
	tests := []struct {
		method   string
		wantCode int
	}{
		{
			method:   http.MethodPut,
			wantCode: 405,
		},
		{
			method:   http.MethodHead,
			wantCode: 400,
		},
		{
			method:   http.MethodDelete,
			wantCode: 405,
		},
	}

	ts, stop := prepareTestServer(nil)
	defer stop()
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			resp, _ := doTestRequest(t, ts, tt.method, "/", strings.NewReader("https://example.com/"))
			resp.Body.Close()
			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
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
			name:  "no body provided",
			body:  nil,
			isErr: true,
		},
	}

	ts, stop := prepareTestServer(nil)
	defer stop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := doTestRequest(t, ts, http.MethodPost, "/", tt.body)
			resp.Body.Close()
			if tt.isErr {
				assert.Equal(t, 400, resp.StatusCode)
			} else {
				assert.Equal(t, 201, resp.StatusCode)
			}
		})
	}
}

func TestShortenEndpointSupportsCustomizableBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		configURL string
		want      string
	}{
		{
			name:      "http scheme",
			configURL: "http://example.com",
			want:      "http://example.com/",
		},
		{
			name:      "localhost with port",
			configURL: "http://localhost:8080",
			want:      "http://localhost:8080/",
		},
		{
			name:      "https scheme",
			configURL: "https://example.com",
			want:      "https://example.com/",
		},
		{
			name:      "leading slash included",
			configURL: "https://example.com/",
			want:      "https://example.com/",
		},
		{
			name:      "custom path",
			configURL: "https://example.com/link",
			want:      "https://example.com/link/",
		},
		{
			name:      "custom path with leading slash",
			configURL: "https://example.com/link/",
			want:      "https://example.com/link/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testBaseURL, _ := url.Parse(tt.configURL)
			ts, stop := prepareTestServer(&config.Config{BaseURL: testBaseURL})
			defer stop()
			resp, body := doTestRequest(t, ts, http.MethodPost, "/", strings.NewReader("https://ya.ru/"))
			resp.Body.Close()
			assert.Equal(t, tt.want, body[:len(body)-7])
		})
	}
}

func TestAPIShortenAndExpandURLs(t *testing.T) {
	TestURLs := []string{
		"https://ya.ru",
		"https://practicum.yandex.ru/learn/go-developer/",
		"https://www.google.com/search?q=golang&client=safari&ei=k3jbYbeDNMOxrgT-ha3gBA&start=10&sa=N&ved=2ahUKEwj3mPjk8aX1AhXDmIsKHf5CC0wQ8tMDegQIAhA5&biw=1280&bih=630&dpr=2",
	}

	type ShortenReqBody struct {
		URL string `json:"url"`
	}
	type ShortenResultBody struct {
		Result string `json:"result"`
	}

	ts, stop := prepareTestServer(nil)
	defer stop()
	for _, TestURL := range TestURLs {
		reqJSON, _ := json.Marshal(&ShortenReqBody{URL: TestURL})
		resp, body := doTestRequest(t, ts, http.MethodPost, "/api/shorten", bytes.NewReader(reqJSON))
		resp.Body.Close()

		assert.Equal(t, 201, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		// Получаем относительный url, состоящий из 7 символов, и пробуем перейти по нему
		var resultJSON ShortenResultBody
		json.Unmarshal([]byte(body), &resultJSON)
		parsed, _ := url.Parse(resultJSON.Result)
		assert.Len(t, strings.Trim(parsed.Path, "/"), 7)
		resp, _ = doTestRequest(t, ts, http.MethodGet, parsed.Path, nil)
		resp.Body.Close()
		// Получем ожидаем редирект на оригинальный url
		assert.Equal(t, 307, resp.StatusCode)
		assert.Equal(t, TestURL, resp.Header.Get("Location"))
	}
}

func TestAPIShortenURLUnsupportedHTTPMethods(t *testing.T) {
	tests := []struct {
		method   string
		wantCode int
	}{
		{
			method:   http.MethodPut,
			wantCode: 404,
		},
		{
			method:   http.MethodHead,
			wantCode: 400,
		},
		{
			method:   http.MethodDelete,
			wantCode: 404,
		},
	}

	ts, stop := prepareTestServer(nil)
	defer stop()
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			resp, _ := doTestRequest(t, ts, tt.method, "/api/shorten/", strings.NewReader(`{"url": "https://example.com/"}`))
			resp.Body.Close()
			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}

func TestAPIShortenEndpointRequiresValidJSON(t *testing.T) {
	tests := []struct {
		name  string
		body  io.Reader
		isErr bool
	}{
		{
			name:  "positive case",
			body:  strings.NewReader(`{"url": "https://example.com/"}`),
			isErr: false,
		},
		{
			name:  "empty body",
			body:  strings.NewReader(""),
			isErr: true,
		},
		{
			name:  "no body provided",
			body:  nil,
			isErr: true,
		},
		{
			name:  "empty json",
			body:  strings.NewReader("{}"),
			isErr: true,
		},
		{
			name:  "bad json field name",
			body:  strings.NewReader(`{"link": "https://example.com/"}`),
			isErr: true,
		},
		{
			name:  "null json",
			body:  strings.NewReader("null"),
			isErr: true,
		},
		{
			name:  "invalid json",
			body:  strings.NewReader(`{url: "https://example.com/"}`),
			isErr: true,
		},
		{
			name:  "url is empty",
			body:  strings.NewReader(`{"url": ""`),
			isErr: true,
		},
	}

	ts, stop := prepareTestServer(nil)
	defer stop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := doTestRequest(t, ts, http.MethodPost, "/api/shorten", tt.body)
			resp.Body.Close()
			if tt.isErr {
				assert.Equal(t, 400, resp.StatusCode)
			} else {
				assert.Equal(t, 201, resp.StatusCode)
			}
		})
	}
}

func TestExpandEndpointRequiresShortURLID(t *testing.T) {
	ts, stop := prepareTestServer(nil)
	defer stop()
	resp, _ := doTestRequest(t, ts, http.MethodGet, "/", nil)
	resp.Body.Close()
	assert.Equal(t, 405, resp.StatusCode)
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
			name:   "empty short url id",
			req:    "//",
			isErr:  true,
			result: "",
		},
		{
			name:   "unknown short url",
			req:    "/foobar",
			isErr:  true,
			result: "",
		},
	}

	shorterner, _ := app.New(&config.Config{})
	shorterner.Storage.Set("gogogo", "https://go.dev/")
	ts := httptest.NewServer(router.New(shorterner))
	defer ts.Close()
	defer shorterner.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := doTestRequest(t, ts, http.MethodGet, tt.req, nil)
			resp.Body.Close()
			if tt.isErr {
				assert.Equal(t, 404, resp.StatusCode)
			} else {
				assert.Equal(t, 307, resp.StatusCode)
				assert.Equal(t, tt.result, resp.Header.Get("Location"))
			}
		})
	}
}
