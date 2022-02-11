package middleware_test

import (
	"bytes"
	"compress/gzip"
	"github.com/sergeii/practikum-go-url-shortener/internal/middleware"
	mwtest "github.com/sergeii/practikum-go-url-shortener/pkg/testing/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func HelloNameHandler(w http.ResponseWriter, r *http.Request) {
	name, _ := io.ReadAll(r.Body)
	w.Write([]byte("Hello, " + string(name)))
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
	rr := mwtest.RequestWithMiddleware(HelloNameHandler, middleware.GzipSupport, req)

	require.Equal(t, rr.Code, 200)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, rr.Body.String(), "Hello, John")
}

func TestGzipSupportNoEncodeDecode(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", strings.NewReader("John"))
	req.Header.Set("Accept-Encoding", "Accept-Encoding: deflate, gzip")
	rr := mwtest.RequestWithMiddleware(HelloNameHandler, middleware.GzipSupport, req)

	require.Equal(t, rr.Code, 200)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, gunzipText(rr.Body), "Hello, John")
}

func TestGzipSupportEncodeNoDecode(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", gzipBuffer([]byte("John")))
	req.Header.Set("Content-Encoding", "gzip")
	rr := mwtest.RequestWithMiddleware(HelloNameHandler, middleware.GzipSupport, req)

	require.Equal(t, rr.Code, 200)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, rr.Body.String(), "Hello, John")
}

func TestGzipSupportEncodeDecode(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", gzipBuffer([]byte("John")))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "Accept-Encoding: deflate, gzip")
	rr := mwtest.RequestWithMiddleware(HelloNameHandler, middleware.GzipSupport, req)

	require.Equal(t, rr.Code, 200)
	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, gunzipText(rr.Body), "Hello, John")
}

func TestGzipNotAccepted(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", gzipBuffer([]byte("John")))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "Accept-Encoding: deflate")
	rr := mwtest.RequestWithMiddleware(HelloNameHandler, middleware.GzipSupport, req)

	require.Equal(t, rr.Code, 200)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
	assert.Equal(t, rr.Body.String(), "Hello, John")
}

func TestGzipIsDeclaredButNotProvided(t *testing.T) {
	req, _ := http.NewRequest("POST", "/", strings.NewReader("John"))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "Accept-Encoding: gzip")
	rr := mwtest.RequestWithMiddleware(HelloNameHandler, middleware.GzipSupport, req)

	require.Equal(t, rr.Code, 415)
	assert.Equal(t, "", rr.Header().Get("Content-Encoding"))
}
