// Package skill provides Sideref HTTP middleware for capability-gated remote providers.
//
// This middleware verifies OCAPN Sideref tokens on incoming HTTP requests before
// forwarding to a backend inference server (e.g., vLLM serving Devstral).
//
// Wire format: compact-marshaled SiderefToken in X-Sideref-Token header (hex-encoded).
// The middleware recomputes the HMAC against the server's device secret and rejects
// forged, expired, or mismatched tokens with constant-time comparison.
//
// Usage:
//
//	backend := httputil.NewSingleHostReverseProxy(vllmURL)
//	protected := SiderefMiddleware(backend, deviceSecret, SiderefMiddlewareOpts{})
//	http.ListenAndServe(":8080", protected)
package skill

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// SiderefHeader is the HTTP header carrying the compact-marshaled Sideref token.
	SiderefHeader = "X-Sideref-Token"

	// SiderefSkillHeader optionally declares which skill is being invoked.
	// If present, the middleware checks it against the token's SkillName.
	SiderefSkillHeader = "X-Sideref-Skill"
)

// SiderefMiddlewareOpts configures the middleware behavior.
type SiderefMiddlewareOpts struct {
	// AllowedSkills restricts which skill names may be invoked.
	// Empty means all skills are allowed (token still verified).
	AllowedSkills []string

	// MinTokenVersion rejects tokens below this version (revocation).
	MinTokenVersion uint8

	// Logger receives auth events. Nil = log.Printf.
	Logger func(format string, args ...interface{})

	// RateLimit per-token requests per second. 0 = no limit.
	RateLimitRPS int
}

// SiderefMiddleware returns an http.Handler that verifies Sideref tokens
// before forwarding requests to the next handler (typically a reverse proxy).
func SiderefMiddleware(next http.Handler, deviceSecret [16]byte, opts SiderefMiddlewareOpts) http.Handler {
	logf := opts.Logger
	if logf == nil {
		logf = log.Printf
	}

	allowedSet := make(map[string]bool, len(opts.AllowedSkills))
	for _, s := range opts.AllowedSkills {
		allowedSet[s] = true
	}

	var rl *rateLimiter
	if opts.RateLimitRPS > 0 {
		rl = newRateLimiter(opts.RateLimitRPS)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health check bypass
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract token from header
		tokenHex := r.Header.Get(SiderefHeader)
		if tokenHex == "" {
			siderefError(w, http.StatusUnauthorized, "missing X-Sideref-Token header")
			logf("[sideref] REJECT %s %s: missing token", r.Method, r.URL.Path)
			return
		}

		// Decode hex → compact bytes
		tokenBytes, err := hex.DecodeString(strings.TrimSpace(tokenHex))
		if err != nil {
			siderefError(w, http.StatusBadRequest, "invalid X-Sideref-Token: not valid hex")
			logf("[sideref] REJECT %s %s: bad hex encoding", r.Method, r.URL.Path)
			return
		}

		// Unmarshal compact format
		token, err := UnmarshalSideref(tokenBytes)
		if err != nil {
			siderefError(w, http.StatusBadRequest, fmt.Sprintf("invalid X-Sideref-Token: %v", err))
			logf("[sideref] REJECT %s %s: unmarshal failed: %v", r.Method, r.URL.Path, err)
			return
		}

		// Check skill allowlist
		if len(allowedSet) > 0 && !allowedSet[token.SkillName] {
			siderefError(w, http.StatusForbidden, fmt.Sprintf("skill %q not allowed on this provider", token.SkillName))
			logf("[sideref] REJECT %s %s: skill %q not in allowlist", r.Method, r.URL.Path, token.SkillName)
			return
		}

		// Check optional skill header matches token
		if skillHeader := r.Header.Get(SiderefSkillHeader); skillHeader != "" {
			if skillHeader != token.SkillName {
				siderefError(w, http.StatusBadRequest, "X-Sideref-Skill does not match token skill name")
				logf("[sideref] REJECT %s %s: skill header mismatch (%q vs %q)", r.Method, r.URL.Path, skillHeader, token.SkillName)
				return
			}
		}

		// Check minimum token version (revocation)
		if token.TokenVersion < opts.MinTokenVersion {
			siderefError(w, http.StatusForbidden, fmt.Sprintf("token version %d below minimum %d (revoked)", token.TokenVersion, opts.MinTokenVersion))
			logf("[sideref] REJECT %s %s: token version %d < min %d", r.Method, r.URL.Path, token.TokenVersion, opts.MinTokenVersion)
			return
		}

		// Verify HMAC (constant-time comparison, checks expiration)
		if err := VerifySideref(token, token.SkillName, deviceSecret); err != nil {
			siderefError(w, http.StatusForbidden, fmt.Sprintf("token verification failed: %v", err))
			logf("[sideref] REJECT %s %s: verify failed for %q: %v", r.Method, r.URL.Path, token.SkillName, err)
			return
		}

		// Rate limiting per skill
		if rl != nil && !rl.allow(token.SkillName) {
			siderefError(w, http.StatusTooManyRequests, "rate limit exceeded")
			logf("[sideref] RATELIMIT %s %s: skill %q", r.Method, r.URL.Path, token.SkillName)
			return
		}

		// Token verified — forward to backend
		logf("[sideref] ALLOW %s %s: skill=%q version=%d", r.Method, r.URL.Path, token.SkillName, token.TokenVersion)

		// Set verified skill name for downstream handlers
		r.Header.Set("X-Verified-Skill", token.SkillName)
		r.Header.Set("X-Verified-Trit", fmt.Sprintf("%d", ComputeTriEmbedded(token.SkillName)))

		next.ServeHTTP(w, r)
	})
}

// siderefError writes a JSON error response.
func siderefError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q,"status":%d}`, msg, status)
}

// --- simple token-bucket rate limiter per skill ---

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rps     int
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

func newRateLimiter(rps int) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*bucket),
		rps:     rps,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(rl.rps), lastCheck: now}
		rl.buckets[key] = b
	}

	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * float64(rl.rps)
	if b.tokens > float64(rl.rps) {
		b.tokens = float64(rl.rps)
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
