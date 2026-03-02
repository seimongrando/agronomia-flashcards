// Package push implements Web Push notifications (RFC 8291) using only Go
// standard library — no external dependencies required.
//
// Key concepts:
//   - VAPID (RFC 8292): JWT signed with ECDSA P-256 to authenticate the server.
//   - Payload encryption (RFC 8291 + RFC 8188): ECDH key agreement → AES-128-GCM.
package push

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// GenerateVAPIDKeys generates a P-256 VAPID key pair.
// Returns (privateKeyBase64, publicKeyBase64) for storage in environment variables.
// Invoke once and persist the result.
func GenerateVAPIDKeys() (private, public string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate vapid key: %w", err)
	}
	// Private: raw 32-byte scalar (D).
	priv := padTo32(key.D.Bytes())
	// Public: uncompressed 65-byte point (0x04 || X || Y).
	pub := elliptic.Marshal(elliptic.P256(), key.X, key.Y)
	return base64.RawURLEncoding.EncodeToString(priv),
		base64.RawURLEncoding.EncodeToString(pub), nil
}

// LoadPrivateKey decodes a base64url-encoded VAPID private key.
func LoadPrivateKey(b64 string) (*ecdsa.PrivateKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("vapid private key decode: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("vapid private key: expected 32 bytes, got %d", len(raw))
	}
	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: elliptic.P256()},
		D:         new(big.Int).SetBytes(raw),
	}
	priv.X, priv.Y = elliptic.P256().ScalarBaseMult(raw)
	return priv, nil
}

// VAPIDToken generates a signed JWT for the given push endpoint's origin.
// The token is valid for 12 h. subject is a mailto: or https: contact URI.
func VAPIDToken(priv *ecdsa.PrivateKey, audience, subject string) (string, error) {
	now := time.Now().Unix()
	header := mustB64JSON(map[string]string{"alg": "ES256", "typ": "JWT"})
	claims := mustB64JSON(map[string]any{
		"aud": audience,
		"exp": now + 43200,
		"sub": subject,
	})
	sigInput := header + "." + claims

	hash := sha256.Sum256([]byte(sigInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return "", fmt.Errorf("vapid sign: %w", err)
	}

	// IEEE P1363 signature: r || s, each zero-padded to 32 bytes.
	sig := make([]byte, 64)
	copy(sig[32-len(r.Bytes()):32], r.Bytes())
	copy(sig[64-len(s.Bytes()):64], s.Bytes())

	return sigInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// ─── helpers ───────────────────────────────────────────────────────────────

func mustB64JSON(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

func padTo32(b []byte) []byte {
	if len(b) == 32 {
		return b
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}
