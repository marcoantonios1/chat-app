package client

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	identityPubFile  = "identity_ed25519.pub"
	identityPrivFile = "identity_ed25519.key"
)

var (
	keyMu      sync.RWMutex
	cachedPub  []byte
	cachedPriv []byte

	identityMu   sync.RWMutex
	identityPub  []byte
	identityPriv []byte
)

// SaveKeyPair saves the public and private key bytes to files and caches them in memory.
func SaveKeyPair(pub, priv []byte) error {
	dir := getKeyDir()
	if err := os.MkdirAll(dir, 0o700); err != nil { // restrict access
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	pubPath := filepath.Join(dir, "public.key")
	privPath := filepath.Join(dir, "private.key")

	// write files with safe permissions
	if err := os.WriteFile(pubPath, pub, 0o600); err != nil {
		return fmt.Errorf("failed to save public key: %w", err)
	}
	if err := os.WriteFile(privPath, priv, 0o600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	// update in-memory cache
	keyMu.Lock()
	cachedPub = append([]byte(nil), pub...)   // copy
	cachedPriv = append([]byte(nil), priv...) // copy
	keyMu.Unlock()

	return nil
}

// LoadKeyPair loads the public and private key bytes from files and caches them.
// If keys are already cached, returns them directly.
func LoadKeyPair() ([]byte, []byte, error) {
	// fast path: return cached copy
	keyMu.RLock()
	if len(cachedPub) != 0 || len(cachedPriv) != 0 {
		pub := append([]byte(nil), cachedPub...)
		priv := append([]byte(nil), cachedPriv...)
		keyMu.RUnlock()
		return pub, priv, nil
	}
	keyMu.RUnlock()

	// load from disk
	dir := getKeyDir()
	pubPath := filepath.Join(dir, "public.key")
	privPath := filepath.Join(dir, "private.key")

	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read public key: %w", err)
	}

	priv, err := os.ReadFile(privPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// cache them
	keyMu.Lock()
	cachedPub = append([]byte(nil), pub...)
	cachedPriv = append([]byte(nil), priv...)
	keyMu.Unlock()

	return append([]byte(nil), pub...), append([]byte(nil), priv...), nil
}

// GetKeyPair is a convenience wrapper that tries cache first, then disk.
func GetKeyPair() ([]byte, []byte, error) {
	return LoadKeyPair()
}

// ClearKeyCache wipes the in-memory cached keys.
func ClearKeyCache() {
	keyMu.Lock()
	cachedPub = nil
	cachedPriv = nil
	keyMu.Unlock()
}

// getKeyDir returns a directory path to store keys securely.
func getKeyDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop", ".chatkeys")
}

// GetIdentityKeyPair returns (pub, priv, error). It loads from cache/disk.
func GetIdentityKeyPair() ([]byte, []byte, error) {
	identityMu.RLock()
	if len(identityPub) > 0 && len(identityPriv) > 0 {
		pub := append([]byte(nil), identityPub...)
		priv := append([]byte(nil), identityPriv...)
		identityMu.RUnlock()
		return pub, priv, nil
	}
	identityMu.RUnlock()
	return LoadIdentityKeyPair()
}

// GenerateIdentityKeyPair creates a new ed25519 keypair (pub, priv).
func GenerateIdentityKeyPair() ([]byte, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate identity key: %w", err)
	}
	return pub, priv, nil
}

// SaveIdentityKeyPair writes identity keys to disk and caches them.
func SaveIdentityKeyPair(pub, priv []byte) error {
	dir := getKeyDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir key dir: %w", err)
	}
	pubPath := filepath.Join(dir, identityPubFile)
	privPath := filepath.Join(dir, identityPrivFile)

	if err := os.WriteFile(pubPath, pub, 0600); err != nil {
		return fmt.Errorf("write identity pub: %w", err)
	}
	if err := os.WriteFile(privPath, priv, 0600); err != nil {
		return fmt.Errorf("write identity priv: %w", err)
	}

	identityMu.Lock()
	identityPub = append([]byte(nil), pub...)
	identityPriv = append([]byte(nil), priv...)
	identityMu.Unlock()
	return nil
}

func LoadIdentityKeyPair() ([]byte, []byte, error) {
	dir := getKeyDir()
	pubPath := filepath.Join(dir, identityPubFile)
	privPath := filepath.Join(dir, identityPrivFile)

	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read identity pub: %w", err)
	}
	priv, err := os.ReadFile(privPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read identity priv: %w", err)
	}

	identityMu.Lock()
	identityPub = append([]byte(nil), pub...)
	identityPriv = append([]byte(nil), priv...)
	identityMu.Unlock()
	return append([]byte(nil), pub...), append([]byte(nil), priv...), nil
}
