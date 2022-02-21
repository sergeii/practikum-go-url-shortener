package hasher_test

import (
	"testing"

	"github.com/sergeii/practikum-go-url-shortener/pkg/url/hasher"
	"github.com/stretchr/testify/assert"
)

func TestURLWithSha256(t *testing.T) {
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
	theHasher := hasher.NewNaiveHasher()
	for _, tt := range tests {
		val := theHasher.Hash(tt.URL)
		assert.Equal(t, tt.want, val)
	}
}
