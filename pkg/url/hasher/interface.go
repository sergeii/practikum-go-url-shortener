package hasher

type Hasher interface {
	Hash(string) string
}
