package client

import (
	"fmt"

	"github.com/cloudflare/circl/kem/kyber/kyber1024"
)

// GenerateKeyPair returns (publicKeyBytes, privateKeyBytes, error)
func GenerateKyberKeyPair() ([]byte, []byte, error) {
	scheme := kyber1024.Scheme() // get the Kyber scheme implementation
	pub, priv, err := scheme.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("GenerateKeyPair failed: %w", err)
	}
	// Marshal public and private to bytes (binary representation)
	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("public.MarshalBinary error: %w", err)
	}
	privBytes, err := priv.MarshalBinary()
	if err != nil {
		return nil, nil, fmt.Errorf("private.MarshalBinary error: %w", err)
	}
	return pubBytes, privBytes, nil
}

// EncapsulateWithPub uses the recipient’s public key bytes to encapsulate
// a shared key and produce a ciphertext
func EncapsulateWithPub(pubBytes []byte) ([]byte, []byte, error) {
	scheme := kyber1024.Scheme()
	pub, err := scheme.UnmarshalBinaryPublicKey(pubBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal public key")
	}
	ct, shared, err := pub.Scheme().Encapsulate(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("Encapsulate error: %w", err)
	}
	return ct, shared, nil
}

// DecapsulateWithPriv uses your private key bytes and ciphertext to recover shared key
func DecapsulateWithPriv(privBytes, ciphertext []byte) ([]byte, error) {
	scheme := kyber1024.Scheme()
	priv, err := scheme.UnmarshalBinaryPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key")
	}
	shared, err := priv.Scheme().Decapsulate(priv, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("Decapsulate error: %w", err)
	}
	return shared, nil
}

// helper to create deterministic salt for HKDF
func makeSalt(a, b string) []byte {
	if a < b {
		return []byte("chat-client-salt:" + a + ":" + b)
	}
	return []byte("chat-client-salt:" + b + ":" + a)
}