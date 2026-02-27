package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// TieredConfig maps a URL-path prefix to its own rate-limit parameters.
// The first matching entry wins; prefixes are checked in order.
// RPS is requests-per-second as a float64; it is converted to rate.Limit internally.
type TieredConfig struct {
	Prefix string
	RPS    float64
	Burst  int
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type prefixStore struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
	rps     rate.Limit
	burst   int
}

func (s *prefixStore) get(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[ip]; ok {
		e.lastSeen = time.Now()
		return e.limiter
	}
	l := rate.NewLimiter(s.rps, s.burst)
	s.entries[ip] = &ipEntry{limiter: l, lastSeen: time.Now()}
	return l
}

// RateLimiterStore holds one per-IP limiter pool for each configured prefix.
type RateLimiterStore struct {
	tiers []*struct {
		prefix string
		store  *prefixStore
	}
}

// NewRateLimiterStore creates a store with a single global pool (legacy, kept for
// backward compatibility). Use NewTieredRateLimiterStore for tiered control.
func NewRateLimiterStore(rps float64, burst int) *RateLimiterStore {
	return NewTieredRateLimiterStore([]TieredConfig{
		{Prefix: "/", RPS: rps, Burst: burst},
	})
}

// NewTieredRateLimiterStore creates a store with independent per-IP limit pools
// for each URL-prefix tier.
func NewTieredRateLimiterStore(tiers []TieredConfig) *RateLimiterStore {
	s := &RateLimiterStore{}
	for _, t := range tiers {
		ps := &prefixStore{
			entries: make(map[string]*ipEntry),
			rps:     rate.Limit(t.RPS),
			burst:   t.Burst,
		}
		go runCleanup(ps)
		s.tiers = append(s.tiers, &struct {
			prefix string
			store  *prefixStore
		}{prefix: t.Prefix, store: ps})
	}
	return s
}

func runCleanup(s *prefixStore) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		for ip, e := range s.entries {
			if time.Since(e.lastSeen) > 3*time.Minute {
				delete(s.entries, ip)
			}
		}
		s.mu.Unlock()
	}
}

func (s *RateLimiterStore) allow(ip, path string) bool {
	for _, t := range s.tiers {
		if strings.HasPrefix(path, t.prefix) {
			return t.store.get(ip).Allow()
		}
	}
	return true
}

// RateLimit returns a middleware that enforces the tiered limits.
// Paths that do not match any tier are always allowed.
func RateLimit(store *RateLimiterStore) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !store.allow(ip, r.URL.Path) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"type":"about:blank","title":"Too Many Requests","status":429}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the real client IP for rate-limiting purposes.
//
// When behind a single trusted reverse proxy (e.g. Render, nginx), the proxy
// appends the client's real IP to X-Forwarded-For, so the rightmost entry is
// the one added by the trusted infrastructure and cannot be spoofed by a
// client pre-setting the header. Taking the leftmost value (the common naive
// approach) would let attackers bypass rate limits by sending a forged header.
//
// If there is no proxy in front of the app the header is absent and we fall
// back to RemoteAddr, which is always the true peer IP.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		// Rightmost non-empty part — added by the nearest trusted proxy.
		for i := len(parts) - 1; i >= 0; i-- {
			if ip := strings.TrimSpace(parts[i]); ip != "" {
				return ip
			}
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
