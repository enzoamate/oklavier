package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const encryptedPrefix = "enc:"

// Encrypt encrypts plaintext using AES-256-GCM and returns a string
// prefixed with "enc:" followed by base64-encoded ciphertext.
// Returns empty string for empty input.
func Encrypt(plaintext, key string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value previously encrypted with Encrypt.
// If the value does not have the "enc:" prefix, it is returned as-is
// (backwards compatibility with legacy plaintext data).
func Decrypt(ciphertext, key string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, encryptedPrefix) {
		return ciphertext, nil // Not encrypted (legacy data), return as-is
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, encryptedPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// IsEncrypted checks whether a string has the "enc:" prefix.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encryptedPrefix)
}

// deriveKey pads or truncates the key to exactly 32 bytes for AES-256.
func deriveKey(key string) []byte {
	k := make([]byte, 32)
	copy(k, []byte(key))
	return k
}
