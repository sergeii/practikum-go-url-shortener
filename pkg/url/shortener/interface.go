package shortener

type Shortener interface {
	Shorten(string) string
}
