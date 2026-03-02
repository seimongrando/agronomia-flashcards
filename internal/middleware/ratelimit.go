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
	// trustedProxy controls whether X-Forwarded-For / X-Real-IP headers are
	// trusted for client IP resolution. Set true only when behind a known proxy.
	trustedProxy bool
}

// NewRateLimiterStore creates a store with a single global pool (legacy, kept for
// backward compatibility). Use NewTieredRateLimiterStore for tiered control.
func NewRateLimiterStore(rps float64, burst int) *RateLimiterStore {
	return NewTieredRateLimiterStore([]TieredConfig{
		{Prefix: "/", RPS: rps, Burst: burst},
	})
}

// NewTieredRateLimiterStore creates a store with independent per-IP limit pools
// for each URL-prefix tier. trustedProxy controls whether forwarded-IP headers
// are honoured (set true when behind a single known reverse proxy like Render).
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

// SetTrustedProxy configures whether forwarded-IP headers are trusted.
// Call this after NewTieredRateLimiterStore.
func (s *RateLimiterStore) SetTrustedProxy(v bool) { s.trustedProxy = v }

// RateLimit returns a middleware that enforces the tiered limits.
// Paths that do not match any tier are always allowed.
func RateLimit(store *RateLimiterStore) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r, store.trustedProxy)
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
// When trusted=true (behind a single known reverse proxy such as Render or nginx),
// the proxy appends the real client IP to X-Forwarded-For. Taking the rightmost
// non-empty value is safe because that entry is added by the trusted proxy and
// cannot be spoofed by a client pre-setting the header.
//
// When trusted=false (direct access or local dev), forwarded headers are ignored
// entirely and we use RemoteAddr — the true TCP peer — to prevent IP spoofing.
func clientIP(r *http.Request, trusted bool) string {
	if trusted {
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
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
