package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

func WithMiddleware(h http.Handler, middlewares ...Middleware) http.Handler {
	for _, middleware := range middlewares {
		h = middleware(h)
	}
	return h
}
