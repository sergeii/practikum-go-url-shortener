package sign

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"hash"
)

type Signer struct {
	secret []byte
	hasher hash.Hash
}

func New(secret []byte) *Signer {
	return &Signer{
		secret: secret,
		hasher: hmac.New(sha256.New, secret),
	}
}

func (s *Signer) Sign(data []byte) []byte {
	s.hasher.Write(data)
	signature := s.hasher.Sum(nil)
	s.hasher.Reset()
	return signature
}

func (s *Signer) Sign64(data []byte) string {
	signature := s.Sign(data)
	return base64.StdEncoding.EncodeToString(signature)
}

func (s *Signer) Verify(data, actualSig []byte) bool {
	expectedSig := s.Sign(data)
	return bytes.Equal(expectedSig, actualSig)
}

func (s *Signer) Verify64(data []byte, actualSig64 string) (bool, error) {
	actualSig, err := base64.StdEncoding.DecodeString(actualSig64)
	if err != nil {
		return false, err
	}
	expectedSig := s.Sign(data)
	return bytes.Equal(expectedSig, actualSig), nil
}
