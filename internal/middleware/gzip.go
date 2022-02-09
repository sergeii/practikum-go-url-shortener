package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func GzipSupport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получили запрос, закодированный в gip - оборачиваем Body в gzip.Reader
		// для того чтобы читатели Body могли считывать тело запроса прозрачно
		if r.Header.Get("Content-Encoding") == "gzip" {
			rawBody := r.Body
			defer rawBody.Close()
			gzReader, err := gzip.NewReader(rawBody)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
				return
			}
			r.Body = gzReader
			defer gzReader.Close()
		}

		// Если клиент не поддерживает gzip, то передаем управление дальше
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Оборачиваем ResponseWriter в gzip.Writer для прозрачной поточной записи-сжатия
		gzWriter, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer gzWriter.Close()

		w.Header().Set("Content-Encoding", "gzip")
		// передаём обработчику страницы переменную типа gzipResponseWriter для вывода данных
		next.ServeHTTP(gzipResponseWriter{ResponseWriter: w, Writer: gzWriter}, r)
	})
}
