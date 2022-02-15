package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/handlers"
	"github.com/sergeii/practikum-go-url-shortener/internal/middleware"
	"github.com/sergeii/practikum-go-url-shortener/internal/router"
	"github.com/sergeii/practikum-go-url-shortener/pkg/security/sign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setAuthCookie(r *http.Request, secretKey []byte, userID string) *http.Cookie {
	signer := sign.New(secretKey)
	signature64 := signer.Sign64([]byte(userID))
	cookieValue := userID + ":" + signature64
	cookieExpiresAt := time.Now().Add(middleware.AuthUserCookieExpiration)
	cookie := &http.Cookie{
		Name:    middleware.AuthUserCookieName,
		Value:   cookieValue,
		Expires: cookieExpiresAt,
	}
	if r != nil {
		r.AddCookie(cookie)
	}
	return cookie
}

func prepareTestServer(t *testing.T, overrides ...app.Override) (*httptest.Server, *app.App) {
	shorterner, err := app.New(overrides...)
	if err != nil {
		panic(err)
	}
	ts := httptest.NewServer(router.New(shorterner))
	t.Cleanup(ts.Close)
	t.Cleanup(shorterner.Close)
	t.Cleanup(shorterner.Cleanup)
	return ts, shorterner
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
		"https://www.google.com/search?q=golang&client=safari&ei=k3jbYbeDNMOxrgT-ha3gBA&start=10" +
			"&sa=N&ved=2ahUKEwj3mPjk8aX1AhXDmIsKHf5CC0wQ8tMDegQIAhA5&biw=1280&bih=630&dpr=2",
	}

	ts, _ := prepareTestServer(t)
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

func TestShortenEndpointHandlesDuplicateURLs(t *testing.T) {
	ts, shorterner := prepareTestServer(t)

	if shorterner.DB == nil {
		t.Skip("Skipping test because it requires db")
	}

	resp, shortURL1 := doTestRequest(t, ts, http.MethodPost, "/", strings.NewReader("https://example.com/"))
	resp.Body.Close()
	assert.Equal(t, 201, resp.StatusCode)

	resp, shortURL2 := doTestRequest(t, ts, http.MethodPost, "/", strings.NewReader("https://example.com/"))
	resp.Body.Close()
	assert.Equal(t, 409, resp.StatusCode)

	assert.Equal(t, shortURL1, shortURL2)

	parsed, _ := url.Parse(shortURL2)
	resp, _ = doTestRequest(t, ts, http.MethodGet, parsed.Path, nil)
	resp.Body.Close()
	assert.Equal(t, 307, resp.StatusCode)
	assert.Equal(t, "https://example.com/", resp.Header.Get("Location"))
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

	ts, _ := prepareTestServer(t)
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

	ts, _ := prepareTestServer(t)
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
			ts, _ := prepareTestServer(t, func(cfg *app.Config) error {
				cfg.BaseURL = testBaseURL
				return nil
			})
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
		"https://www.google.com/search?q=golang&client=safari&ei=k3jbYbeDNMOxrgT-ha3gBA" +
			"&start=10&sa=N&ved=2ahUKEwj3mPjk8aX1AhXDmIsKHf5CC0wQ8tMDegQIAhA5&biw=1280&bih=630&dpr=2",
	}

	type ShortenReqBody struct {
		URL string `json:"url"`
	}
	type ShortenResultBody struct {
		Result string `json:"result"`
	}

	ts, _ := prepareTestServer(t)
	for _, TestURL := range TestURLs {
		reqJSON, _ := json.Marshal(&ShortenReqBody{URL: TestURL}) // nolint:errchkjson
		resp, body := doTestRequest(t, ts, http.MethodPost, "/api/shorten", bytes.NewReader(reqJSON))
		resp.Body.Close()

		assert.Equal(t, 201, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		// Получаем относительный url, состоящий из 7 символов, и пробуем перейти по нему
		var resultJSON ShortenResultBody
		json.Unmarshal([]byte(body), &resultJSON) // nolint:errcheck
		parsed, _ := url.Parse(resultJSON.Result)
		assert.Len(t, strings.Trim(parsed.Path, "/"), 7)
		resp, _ = doTestRequest(t, ts, http.MethodGet, parsed.Path, nil)
		resp.Body.Close()
		// Получем ожидаем редирект на оригинальный url
		assert.Equal(t, 307, resp.StatusCode)
		assert.Equal(t, TestURL, resp.Header.Get("Location"))
	}
}

func TestAPIShortenEndpointHandlesDuplicateURLs(t *testing.T) {
	var resultJSON1, resultJSON2 handlers.APIShortenResult

	ts, shorterner := prepareTestServer(t)
	if shorterner.DB == nil {
		t.Skip("Skipping test because it requires db")
	}

	reqJSON, _ := json.Marshal(&handlers.APIShortenRequest{URL: "https://example.com/"}) // nolint:errchkjson
	resp, body := doTestRequest(t, ts, http.MethodPost, "/api/shorten", bytes.NewReader(reqJSON))
	resp.Body.Close()
	json.Unmarshal([]byte(body), &resultJSON1) // nolint:errcheck

	require.Equal(t, 201, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	reqJSON, _ = json.Marshal(&handlers.APIShortenRequest{URL: "https://example.com/"}) // nolint:errchkjson
	resp, body = doTestRequest(t, ts, http.MethodPost, "/api/shorten", bytes.NewReader(reqJSON))
	resp.Body.Close()
	json.Unmarshal([]byte(body), &resultJSON2) // nolint:errcheck

	require.Equal(t, 409, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	assert.Equal(t, resultJSON1.Result, resultJSON2.Result)

	parsed, _ := url.Parse(resultJSON2.Result)
	resp, _ = doTestRequest(t, ts, http.MethodGet, parsed.Path, nil)
	resp.Body.Close()
	assert.Equal(t, 307, resp.StatusCode)
	assert.Equal(t, "https://example.com/", resp.Header.Get("Location"))
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

	ts, _ := prepareTestServer(t)
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

	ts, _ := prepareTestServer(t)
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
	ts, _ := prepareTestServer(t)
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

	ts, shorterner := prepareTestServer(t)
	shorterner.Storage.Set(context.TODO(), "gogogo", "https://go.dev/", "user1") // nolint: errcheck

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

func TestSetAndGetUserURLS(t *testing.T) {
	ts, shorterner := prepareTestServer(t)
	testURLs := []string{"https://ya.ru", "https://go.dev/"}
	authCookie := setAuthCookie(nil, shorterner.SecretKey, "user1")
	for _, testURL := range testURLs {
		req, _ := http.NewRequest("POST", ts.URL+"/", strings.NewReader(testURL))
		req.AddCookie(authCookie)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		assert.Equal(t, 201, resp.StatusCode)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/user/urls", nil)
	req.AddCookie(authCookie)
	resp, _ := http.DefaultClient.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	jsonItems := make([]handlers.APIUserURLItem, 0)
	json.Unmarshal(body, &jsonItems) // nolint:errcheck
	longUserURLs := make([]string, 0)
	for _, item := range jsonItems {
		longUserURLs = append(longUserURLs, item.OriginalURL)
	}
	assert.ElementsMatch(t, testURLs, longUserURLs)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGetUserURLs(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		URLs   []string
	}{
		{
			name:   "user has 2 urls",
			userID: "user1",
			URLs:   []string{"go", "ya"},
		},
		{
			name:   "user has 1 url",
			userID: "user2",
			URLs:   []string{"imdb"},
		},
		{
			name:   "user has no urls",
			userID: "user3",
			URLs:   []string{},
		},
		{
			name:   "anonymous user has no urls",
			userID: "",
			URLs:   []string{},
		},
	}

	ctx := context.TODO()
	ts, shorterner := prepareTestServer(t)
	shorterner.Storage.Set(ctx, "go", "https://go.dev/", "user1")         // nolint: errcheck
	shorterner.Storage.Set(ctx, "ya", "https://ya.ru/", "user1")          // nolint: errcheck
	shorterner.Storage.Set(ctx, "imdb", "https://www.imdb.com/", "user2") // nolint: errcheck

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", ts.URL+"/user/urls", nil)
			if tt.userID != "" {
				setAuthCookie(req, shorterner.SecretKey, tt.userID)
			}
			resp, _ := http.DefaultClient.Do(req)
			body, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if len(tt.URLs) == 0 {
				assert.Equal(t, 204, resp.StatusCode)
			} else {
				jsonItems := make([]handlers.APIUserURLItem, 0)
				json.Unmarshal(body, &jsonItems) // nolint:errcheck
				userShortURLs := make([]string, 0)
				for _, item := range jsonItems {
					u, _ := url.Parse(item.ShortURL)
					userShortURLs = append(userShortURLs, u.Path[1:])
				}
				assert.ElementsMatch(t, tt.URLs, userShortURLs)
				assert.Equal(t, 200, resp.StatusCode)
			}
		})
	}
}

func TestPingEndpointOK(t *testing.T) {
	ts, _ := prepareTestServer(t)
	resp, _ := doTestRequest(t, ts, http.MethodGet, "/ping", nil)
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestAPIShortenBatchRequest(t *testing.T) {
	ts, _ := prepareTestServer(t)

	TestURLs := map[string]string{
		"foo": "https://ya.ru",
		"bar": "https://practicum.yandex.ru/learn/go-developer/",
		"baz": "https://ya.ru",
		"ham": "https://www.google.com/search?q=golang&client=safari&ei=k3jbYbeDNMOxrgT-ha3gBA" +
			"&start=10&sa=N&ved=2ahUKEwj3mPjk8aX1AhXDmIsKHf5CC0wQ8tMDegQIAhA5&biw=1280&bih=630&dpr=2",
		"eggs": "",
	}

	reqItems := make([]handlers.APIShortenBatchRequestItem, 0, len(TestURLs))
	for corrID, URL := range TestURLs {
		reqItems = append(reqItems, handlers.APIShortenBatchRequestItem{CorrelationID: corrID, OriginalURL: URL})
	}
	reqJSON, _ := json.Marshal(&reqItems) // nolint:errchkjson
	resp, body := doTestRequest(t, ts, http.MethodPost, "/api/shorten/batch", bytes.NewReader(reqJSON))
	resp.Body.Close()

	respItems := make([]handlers.APIShortenBatchResultItem, 0)
	json.Unmarshal([]byte(body), &respItems) // nolint:errcheck
	resultURLs := make(map[string]string)
	for _, item := range respItems {
		resultURLs[item.CorrelationID] = item.ShortURL
	}

	require.Equal(t, 201, resp.StatusCode)
	assert.Len(t, respItems, 4)
	assert.Len(t, resultURLs, 4)
	assert.Contains(t, resultURLs, "foo")
	assert.Contains(t, resultURLs, "bar")
	assert.Contains(t, resultURLs, "baz")
	assert.Contains(t, resultURLs, "ham")

	assert.Equal(t, resultURLs["foo"], resultURLs["baz"])
	parsed, _ := url.Parse(resultURLs["baz"])
	resp, _ = doTestRequest(t, ts, http.MethodGet, parsed.Path, nil)
	resp.Body.Close()
	// Получем ожидаем редирект на оригинальный url
	assert.Equal(t, 307, resp.StatusCode)
	assert.Equal(t, "https://ya.ru", resp.Header.Get("Location"))
}

func TestAPIShortenBatchEmptyBody(t *testing.T) {
	ts, _ := prepareTestServer(t)
	resp, _ := doTestRequest(t, ts, http.MethodPost, "/api/shorten/batch", strings.NewReader("[]"))
	resp.Body.Close()
	require.Equal(t, 400, resp.StatusCode)
}

func TestAPIShortenBatchNoItemsToProcess(t *testing.T) {
	ts, _ := prepareTestServer(t)
	TestURLs := map[string]string{
		"foo": "",
		"bar": "",
	}

	reqItems := make([]handlers.APIShortenBatchRequestItem, 0, len(TestURLs))
	for corrID, URL := range TestURLs {
		reqItems = append(reqItems, handlers.APIShortenBatchRequestItem{CorrelationID: corrID, OriginalURL: URL})
	}
	reqJSON, _ := json.Marshal(&reqItems) // nolint:errchkjson
	resp, _ := doTestRequest(t, ts, http.MethodPost, "/api/shorten/batch", bytes.NewReader(reqJSON))
	resp.Body.Close()
	require.Equal(t, 400, resp.StatusCode)
}
