package middleware_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"github.com/sergeii/practikum-go-url-shortener/internal/app"
	"github.com/sergeii/practikum-go-url-shortener/internal/middleware"
	mwtest "github.com/sergeii/practikum-go-url-shortener/pkg/testing/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func HelloIDHandler(w http.ResponseWriter, r *http.Request) {
	if user, ok := r.Context().Value(middleware.AuthContextKey).(*middleware.AuthUser); ok {
		w.Write([]byte("Hello, " + user.ID))
	} else {
		w.Write([]byte("Hello, anonymous"))
	}
}

func TestNewAuthCookieIsSet(t *testing.T) {
	secretKey, _ := app.GenerateSecretKey(32)
	req, _ := http.NewRequest("POST", "/", nil)
	rr := mwtest.RequestWithMiddleware(HelloIDHandler, middleware.WithAuthentication(secretKey), req)

	cookie := parseAuthSetCookie(rr)
	cookieIsValid, userID := verifyAuthCookie(cookie.Value, secretKey)

	require.Equal(t, rr.Code, 200)
	assert.True(t, userID != "")
	assert.True(t, cookieIsValid)
	assert.Equal(t, rr.Body.String(), "Hello, "+userID)
}

func TestPreviousAuthCookieIsAccepted(t *testing.T) {
	secretKey, _ := app.GenerateSecretKey(32)
	userID := "deadbeef"
	cookieValue := generateAuthCookie(userID, secretKey)
	req, _ := http.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{Name: "auth", Value: cookieValue})
	rr := mwtest.RequestWithMiddleware(HelloIDHandler, middleware.WithAuthentication(secretKey), req)

	require.Equal(t, rr.Code, 200)
	assert.Equal(t, rr.Header().Get("Set-Cookie"), "")
	assert.Equal(t, rr.Body.String(), "Hello, deadbeef")
}

func TestInvalidAuthCookieIsIgnoredNewIsSet(t *testing.T) {
	secretKey, _ := app.GenerateSecretKey(32)
	fakeSecretKey, _ := app.GenerateSecretKey(32)
	tests := []struct {
		name    string
		cookie  *http.Cookie
		isValid bool
	}{
		{
			name:    "positive case",
			cookie:  &http.Cookie{Name: "auth", Value: generateAuthCookie("deadbeef", secretKey)},
			isValid: true,
		},
		{
			name:    "empty cookie value",
			cookie:  &http.Cookie{Name: "auth", Value: ""},
			isValid: false,
		},
		{
			name:    "invalid cookie value #1",
			cookie:  &http.Cookie{Name: "auth", Value: "foobar"},
			isValid: false,
		},
		{
			name:    "invalid cookie value #2",
			cookie:  &http.Cookie{Name: "auth", Value: "foo:bar"},
			isValid: false,
		},
		{
			name:    "invalid cookie value #3",
			cookie:  &http.Cookie{Name: "auth", Value: "foo:bar:baz"},
			isValid: false,
		},
		{
			name: "invalid cookie signature #1",
			cookie: &http.Cookie{
				Name:  "auth",
				Value: "deadbeef:" + base32.StdEncoding.EncodeToString([]byte("foobar")),
			},
			isValid: false,
		},
		{
			name: "invalid cookie signature #2",
			cookie: &http.Cookie{
				Name:  "auth",
				Value: "deadbeef:" + base64.StdEncoding.EncodeToString([]byte("foobar")),
			},
			isValid: false,
		},
		{
			name:    "invalid cookie signature #2",
			cookie:  &http.Cookie{Name: "auth", Value: generateAuthCookie("deadbeef", fakeSecretKey)},
			isValid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/", nil)
			req.AddCookie(tt.cookie)
			rr := mwtest.RequestWithMiddleware(HelloIDHandler, middleware.WithAuthentication(secretKey), req)
			require.Equal(t, rr.Code, 200)
			if tt.isValid {
				assert.Equal(t, rr.Header().Get("Set-Cookie"), "")
				assert.Equal(t, rr.Body.String(), "Hello, deadbeef")
			} else {
				cookie := parseAuthSetCookie(rr)
				cookieIsValid, userID := verifyAuthCookie(cookie.Value, secretKey)
				assert.NotEqual(t, cookie.Value, tt.cookie.Value)
				assert.True(t, userID != "")
				assert.True(t, cookieIsValid)
				assert.Equal(t, rr.Body.String(), "Hello, "+userID)
			}
		})
	}
}

func parseSetCookie(setCookie string) []*http.Cookie {
	resp := &http.Response{Header: http.Header{"Set-Cookie": {setCookie}}}
	return resp.Cookies()
}

func parseAuthSetCookie(rr *httptest.ResponseRecorder) *http.Cookie {
	authCookie := rr.Header().Get("Set-Cookie")
	if authCookie != "" {
		for _, cookie := range parseSetCookie(authCookie) {
			if cookie.Name == "auth" {
				return cookie
			}
		}
	}
	return nil
}

func verifyAuthCookie(cookieValue string, secretKey []byte) (bool, string) {
	cookieParts := strings.Split(cookieValue, ":")
	cookieSig, _ := base64.StdEncoding.DecodeString(cookieParts[1])
	expectedSig := generateAuthCookieSignature(cookieParts[0], secretKey)
	return bytes.Equal(expectedSig, cookieSig), cookieParts[0]
}

func generateAuthCookieSignature(userID string, secretKey []byte) []byte {
	signer := hmac.New(sha256.New, secretKey)
	signer.Write([]byte(userID))
	return signer.Sum(nil)
}

func generateAuthCookie(userID string, secretKey []byte) string {
	sig := generateAuthCookieSignature(userID, secretKey)
	return userID + ":" + base64.StdEncoding.EncodeToString(sig)
}
