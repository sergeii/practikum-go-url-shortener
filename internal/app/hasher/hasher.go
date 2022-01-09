package hasher

type UrlHasher interface {
	HashUrl(string) string
}
