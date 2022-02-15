package middleware

import (
	"net/http"
	"net/http/httptest"
)

func RequestWithMiddleware(
	handlerFunc func(http.ResponseWriter, *http.Request),
	middlewareFunc func(handler http.Handler) http.Handler,
	req *http.Request,
) *httptest.ResponseRecorder {
	mw := middlewareFunc(http.HandlerFunc(handlerFunc))
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	return rr
}
