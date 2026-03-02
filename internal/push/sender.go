package push

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Client sends Web Push notifications using VAPID auth and RFC 8291 encryption.
// All fields are read-only after construction; Client is safe for concurrent use.
type Client struct {
	priv       *ecdsa.PrivateKey
	pubB64     string // base64url-encoded 65-byte uncompressed public key (for VAPID k= header)
	subject    string // VAPID contact URI (mailto: or https:)
	httpClient *http.Client
}

// NewClient builds a Client from base64url-encoded VAPID key pair.
// subject should be a "mailto:" or "https:" URI for the push service to contact.
// Returns nil (without error) when both keys are empty — allows the server to
// start without push configured; callers must guard with client != nil.
func NewClient(vapidPrivB64, vapidPubB64, subject string) (*Client, error) {
	if vapidPrivB64 == "" && vapidPubB64 == "" {
		return nil, nil // push not configured — caller skips notification sending
	}
	priv, err := LoadPrivateKey(vapidPrivB64)
	if err != nil {
		return nil, err
	}
	return &Client{
		priv:       priv,
		pubB64:     vapidPubB64,
		subject:    subject,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// PublicKey returns the base64url-encoded VAPID public key — sent to browsers
// so they can verify the push subscription belongs to this server.
func (c *Client) PublicKey() string { return c.pubB64 }

// SendResult categorises the outcome of a single push attempt.
type SendResult int

const (
	SendOK     SendResult = iota // 201 — delivered to push service
	SendGone                     // 410 — subscription expired; caller should delete
	SendFailed                   // other error
)

// Send encrypts payload and POST it to the push endpoint using VAPID auth.
func (c *Client) Send(endpoint, p256dh, auth string, payload []byte) SendResult {
	// Determine audience (scheme://host) for the VAPID JWT.
	u, err := url.Parse(endpoint)
	if err != nil {
		slog.Warn("push: invalid endpoint", "endpoint", endpoint, "error", err)
		return SendFailed
	}
	audience := u.Scheme + "://" + u.Host

	token, err := VAPIDToken(c.priv, audience, c.subject)
	if err != nil {
		slog.Error("push: vapid token", "error", err)
		return SendFailed
	}

	body, err := Encrypt(payload, p256dh, auth)
	if err != nil {
		slog.Error("push: encrypt", "error", err)
		return SendFailed
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		slog.Error("push: build request", "error", err)
		return SendFailed
	}
	req.Header.Set("Authorization", fmt.Sprintf("vapid t=%s,k=%s", token, c.pubB64))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", "86400")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("push: send failed", "error", err)
		return SendFailed
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		return SendOK
	case http.StatusGone, http.StatusNotFound:
		return SendGone // subscription no longer valid
	default:
		slog.Warn("push: unexpected status", "status", resp.StatusCode, "endpoint", endpoint)
		return SendFailed
	}
}
