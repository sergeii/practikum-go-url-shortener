package hasher_test

import (
	"testing"

	"github.com/sergeii/practikum-go-url-shortener/internal/app/hasher"
	"github.com/stretchr/testify/assert"
)

func TestURLWithSha256(t *testing.T) {
	tests := []struct {
		name string
		URL  string
		want string
	}{
		{
			name: "practicum",
			URL:  "https://practicum.yandex.ru/",
			want: "42b3e75",
		},
		{
			name: "go.dev",
			URL:  "https://go.dev/",
			want: "ba6e07b",
		},
	}
	theHasher := hasher.NewSimpleURLHasher()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := theHasher.HashURL(tt.URL)
			assert.Equal(t, tt.want, val)
		})
	}
}
