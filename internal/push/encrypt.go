package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// Encrypt encrypts payload according to RFC 8291 (Web Push Message Encryption)
// using the aes128gcm content encoding defined in RFC 8188.
//
//   - p256dh: client's P-256 public key, base64url-encoded (65-byte uncompressed point).
//   - auth:   client's auth secret, base64url-encoded (16 bytes).
//
// Returns the complete encrypted body (salt || rs || keyid_len || server_pubkey || ciphertext)
// ready to send as the POST body to the push endpoint.
func Encrypt(payload []byte, p256dh, auth string) ([]byte, error) {
	// Decode client subscription keys.
	clientPubBytes, err := base64.RawURLEncoding.DecodeString(p256dh)
	if err != nil {
		// Some clients omit the standard base64url padding; also try StdEncoding fallback.
		clientPubBytes, err = base64.URLEncoding.DecodeString(p256dh)
		if err != nil {
			return nil, fmt.Errorf("invalid p256dh: %w", err)
		}
	}
	authBytes, err := base64.RawURLEncoding.DecodeString(auth)
	if err != nil {
		authBytes, err = base64.URLEncoding.DecodeString(auth)
		if err != nil {
			return nil, fmt.Errorf("invalid auth: %w", err)
		}
	}

	// Parse client P-256 public key.
	curve := ecdh.P256()
	clientPub, err := curve.NewPublicKey(clientPubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse client public key: %w", err)
	}

	// Generate ephemeral server key pair.
	serverPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	serverPub := serverPriv.PublicKey().Bytes() // 65-byte uncompressed.

	// ECDH shared secret.
	sharedSecret, err := serverPriv.ECDH(clientPub)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	// Generate random 16-byte salt (per record).
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	// ── Key derivation (RFC 8291 §3.3) ────────────────────────────────────
	//
	// PRK_key = HKDF-Extract(auth_secret, ecdh_secret)
	prkKey := hkdfExtract(authBytes, sharedSecret)

	// IKM = HKDF-Expand(PRK_key, "WebPush: info\x00" || ua_pub || as_pub, 32)
	keyInfo := make([]byte, 0, 14+65+65)
	keyInfo = append(keyInfo, []byte("WebPush: info\x00")...)
	keyInfo = append(keyInfo, clientPubBytes...)
	keyInfo = append(keyInfo, serverPub...)
	ikm := hkdfExpand(prkKey, keyInfo, 32)

	// PRK = HKDF-Extract(salt, IKM)
	prk := hkdfExtract(salt, ikm)

	// CEK = HKDF-Expand(PRK, "Content-Encoding: aes128gcm\x00\x01", 16)
	cek := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00\x01"), 16)

	// NONCE = HKDF-Expand(PRK, "Content-Encoding: nonce\x00\x01", 12)
	nonce := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00\x01"), 12)

	// ── Encrypt (RFC 8188 §2.1) ────────────────────────────────────────────
	// Record = plaintext || 0x02  (0x02 = last-record delimiter for aes128gcm)
	record := make([]byte, len(payload)+1)
	copy(record, payload)
	record[len(payload)] = 0x02

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	encrypted := gcm.Seal(nil, nonce, record, nil)

	// ── Build output: salt(16) || rs(4) || keyid_len(1) || serverPub(65) || ciphertext ──
	const recordSize = 4096
	out := make([]byte, 0, 16+4+1+65+len(encrypted))
	out = append(out, salt...)
	rs := make([]byte, 4)
	binary.BigEndian.PutUint32(rs, uint32(recordSize))
	out = append(out, rs...)
	out = append(out, byte(len(serverPub))) // always 65
	out = append(out, serverPub...)
	out = append(out, encrypted...)
	return out, nil
}

// ─── HKDF-SHA-256 (RFC 5869) ──────────────────────────────────────────────

func hkdfExtract(salt, ikm []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	return mac.Sum(nil)
}

func hkdfExpand(prk, info []byte, length int) []byte {
	var t []byte
	okm := make([]byte, 0, length)
	counter := byte(1)
	for len(okm) < length {
		mac := hmac.New(sha256.New, prk)
		mac.Write(t)
		mac.Write(info)
		mac.Write([]byte{counter})
		t = mac.Sum(nil)
		okm = append(okm, t...)
		counter++
	}
	return okm[:length]
}
