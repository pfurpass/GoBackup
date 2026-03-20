package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pbkdf2Iterations = 200_000
	keySize          = 32 // AES-256
)

// DeriveKey derives a 32-byte AES key from password + salt using PBKDF2-SHA256.
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, keySize, sha256.New)
}

// EncryptChunk encrypts plaintext with AES-256-GCM using the given key and nonce.
// Returns ciphertext || tag (GCM appends the 16-byte tag automatically).
func EncryptChunk(plaintext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM: %w", err)
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// DecryptChunk decrypts AES-256-GCM ciphertext.
func DecryptChunk(ciphertext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM: %w", err)
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("GCM Entschlüsselung (falsches Passwort?): %w", err)
	}
	return plain, nil
}
