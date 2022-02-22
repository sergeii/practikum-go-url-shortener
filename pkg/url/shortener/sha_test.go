package shortener_test

import (
	"testing"

	"github.com/sergeii/practikum-go-url-shortener/pkg/url/shortener"
	"github.com/stretchr/testify/assert"
)

func TestShaShortener(t *testing.T) {
	tests := []struct {
		name string
		URL  string
		want string
	}{
		{
			URL:  "https://practicum.yandex.ru/",
			want: "42b3e75",
		},
		{
			URL:  "https://go.dev/",
			want: "ba6e07b",
		},
	}
	theShortener := shortener.NewShaShortener()
	for _, tt := range tests {
		val := theShortener.Shorten(tt.URL)
		assert.Equal(t, tt.want, val)
	}
}
