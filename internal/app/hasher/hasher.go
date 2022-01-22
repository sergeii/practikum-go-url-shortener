package hasher

type URLHasher interface {
	HashURL(string) string
}
