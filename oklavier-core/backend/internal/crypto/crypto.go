package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const encryptedPrefix = "enc:"

// AllowPlaintextDecrypt controls whether Decrypt accepts unprefixed values
// as legacy plaintext. Once `MigrateEncryptCredentials` has run on a fresh
// install, this should be set to false (via env OKLAVIER_ALLOW_PLAINTEXT=0)
// to fail closed: an attacker who can write to the DB cannot strip the
// `enc:` prefix and have the application return the value as plaintext.
func allowPlaintext() bool {
	v := os.Getenv("OKLAVIER_ALLOW_PLAINTEXT")
	return v != "0" && v != "false"
}

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
//
// SECURITY: by default we fail closed when the value is missing the `enc:`
// prefix. The previous behavior (silently returning the value as plaintext)
// let an attacker with DB-write access strip the prefix and downgrade
// encrypted credentials to plaintext on next read. To migrate from a legacy
// install with mixed plaintext rows, set OKLAVIER_ALLOW_PLAINTEXT=1
// temporarily until `MigrateEncryptCredentials` has finished.
func Decrypt(ciphertext, key string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, encryptedPrefix) {
		if allowPlaintext() {
			return ciphertext, nil
		}
		return "", fmt.Errorf("value is not encrypted (refused; set OKLAVIER_ALLOW_PLAINTEXT=1 only during migration)")
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

// deriveKey derives an AES-256 key from the supplied secret using HKDF-SHA256
// with a fixed application label. This avoids the previous "zero-pad or
// truncate to 32 bytes" approach, which (a) had no domain separation from
// the JWT signing key when the same secret was reused for both, and (b)
// produced a low-entropy key when the secret was short.
func deriveKey(key string) []byte {
	const info = "oklavier-aes256gcm-credstore-v1"
	h := hkdf.New(sha256.New, []byte(key), nil /* salt */, []byte(info))
	out := make([]byte, 32)
	if _, err := io.ReadFull(h, out); err != nil {
		// crypto/sha256 + hkdf cannot fail with constant info; if it ever
		// does, we must NOT silently fall back to the unsafe zero-padded key.
		panic(fmt.Sprintf("hkdf derive failed: %v", err))
	}
	return out
}
