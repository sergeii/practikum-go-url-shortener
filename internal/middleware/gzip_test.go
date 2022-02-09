package middleware_test

import (
	"bytes"
	"compress/gzip"
	"github.com/sergeii/practikum-go-url-shortener/internal/middleware"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	name, _ := io.ReadAll(r.Body)
	w.Write([]byte("Hello, " + string(name)))
}

func requestWithMiddleware(req *http.Request) *httptest.ResponseRecorder {
	handler := http.HandlerFunc(HelloHandler)
	mw := middleware.WithMiddleware(handler, middleware.GzipSupport)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	return rr
}

func gzipBuffer(payload []byte) *bytes.Buffer {
	var b bytes.Buffer
	writer := gzip.NewWriter(&b)
	writer.Write(payload)
	writer.Close()
	return &b
}

func gunzipText(buf *bytes.Buffer) string {
	gzReader, _ := gzip.NewReader(buf)
	b, _ := ioutil.ReadAll(gzReader)
	return string(b)
}

func TestGzipSupportNoEncodeNoDecode(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", strings.NewReader("John"))
	rr := requestWithMiddleware(req)

	assert.Equal(t, rr.Code, 200)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, rr.Body.String(), "Hello, John")
}

func TestGzipSupportNoEncodeDecode(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", strings.NewReader("John"))
	req.Header.Set("Accept-Encoding", "Accept-Encoding: deflate, gzip")
	rr := requestWithMiddleware(req)

	assert.Equal(t, rr.Code, 200)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, gunzipText(rr.Body), "Hello, John")
}

func TestGzipSupportEncodeNoDecode(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", gzipBuffer([]byte("John")))
	req.Header.Set("Content-Encoding", "gzip")
	rr := requestWithMiddleware(req)

	assert.Equal(t, rr.Code, 200)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, rr.Body.String(), "Hello, John")
}

func TestGzipSupportEncodeDecode(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", gzipBuffer([]byte("John")))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "Accept-Encoding: deflate, gzip")
	rr := requestWithMiddleware(req)

	assert.Equal(t, rr.Code, 200)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, gunzipText(rr.Body), "Hello, John")
}

func TestGzipNotAccepted(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", gzipBuffer([]byte("John")))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "Accept-Encoding: deflate")
	rr := requestWithMiddleware(req)

	assert.Equal(t, rr.Code, 200)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, rr.Body.String(), "Hello, John")
}

func TestGzipIsDeclaredButNotProvided(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", strings.NewReader("John"))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "Accept-Encoding: gzip")
	rr := requestWithMiddleware(req)

	assert.Equal(t, rr.Code, 415)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
}
