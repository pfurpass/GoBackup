package utils

import (
	"crypto/sha256"
	"hash"
)

// SHA256 returns the SHA-256 digest of data as a byte slice.
func SHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// NewHasher returns a fresh SHA-256 hash.Hash for streaming use.
func NewHasher() hash.Hash {
	return sha256.New()
}
