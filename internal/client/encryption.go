// ...existing code...
package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// Encrypt encrypts plaintext with AES-GCM. Returned string is hex(nonce|ciphertext).
func Encrypt(key, plaintext []byte) (string, error) {
	if l := len(key); l != 32 {
		return "", errors.New("invalid key length: must be 16, 24 or 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	out := append(nonce, ciphertext...)
	return hex.EncodeToString(out), nil
}

// Decrypt expects hex(nonce|ciphertext) produced by Encrypt
func Decrypt(key []byte, ciphertextHex string) (string, error) {
	if l := len(key); l != 32 {
		return "", errors.New("invalid key length: must be 16, 24 or 32 bytes")
	}

	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
