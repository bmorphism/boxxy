package skill

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testDeviceSecret is a fixed secret for test determinism.
var testDeviceSecret = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

// makeTokenHeader creates a hex-encoded compact Sideref token for testing.
func makeTokenHeader(skillName string, secret [16]byte) string {
	token := NewSiderefToken(skillName, secret)
	return hex.EncodeToString(token.MarshalSideref())
}

func makeTokenHeaderWithExpiry(skillName string, secret [16]byte, expiresAt uint32) string {
	token := NewSiderefToken(skillName, secret)
	token = token.WithExpiration(expiresAt)
	return hex.EncodeToString(token.MarshalSideref())
}

func makeTokenHeaderWithVersion(skillName string, secret [16]byte, version uint8) string {
	token := NewSiderefToken(skillName, secret)
	token = token.WithVersion(version)
	return hex.EncodeToString(token.MarshalSideref())
}

// echoHandler is a backend that echoes verified headers.
var echoHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	skill := r.Header.Get("X-Verified-Skill")
	trit := r.Header.Get("X-Verified-Trit")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "skill=%s trit=%s", skill, trit)
})

func TestMiddleware_ValidToken(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, makeTokenHeader("devstral-inference", testDeviceSecret))

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200. body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "skill=devstral-inference") {
		t.Errorf("expected verified skill in response, got %q", rr.Body.String())
	}
}

func TestMiddleware_MissingToken(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	req := httptest.NewRequest("POST", "/v1/completions", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want 401", rr.Code)
	}
}

func TestMiddleware_InvalidHex(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, "not-valid-hex!!!")

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rr.Code)
	}
}

func TestMiddleware_WrongSecret(t *testing.T) {
	wrongSecret := [16]byte{99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99}
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, makeTokenHeader("devstral-inference", wrongSecret))

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403. body: %s", rr.Code, rr.Body.String())
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	pastTime := uint32(time.Now().Unix()) - 3600
	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, makeTokenHeaderWithExpiry("devstral-inference", testDeviceSecret, pastTime))

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "expired") {
		t.Errorf("expected expiration error, got %q", rr.Body.String())
	}
}

func TestMiddleware_FutureExpiry_Allowed(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	futureTime := uint32(time.Now().Unix()) + 3600
	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, makeTokenHeaderWithExpiry("devstral-inference", testDeviceSecret, futureTime))

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rr.Code)
	}
}

func TestMiddleware_AllowedSkills(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{
		AllowedSkills: []string{"devstral-inference", "bci-analysis"},
	})

	// Allowed skill
	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, makeTokenHeader("devstral-inference", testDeviceSecret))
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("allowed skill rejected: status %d", rr.Code)
	}

	// Disallowed skill
	req2 := httptest.NewRequest("POST", "/v1/completions", nil)
	req2.Header.Set(SiderefHeader, makeTokenHeader("unauthorized-skill", testDeviceSecret))
	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusForbidden {
		t.Fatalf("disallowed skill accepted: status %d", rr2.Code)
	}
}

func TestMiddleware_SkillHeaderMismatch(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, makeTokenHeader("devstral-inference", testDeviceSecret))
	req.Header.Set(SiderefSkillHeader, "wrong-skill-name")

	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rr.Code)
	}
}

func TestMiddleware_MinTokenVersion(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{
		MinTokenVersion: 3,
	})

	// Version 2 — below minimum, should reject
	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, makeTokenHeaderWithVersion("devstral-inference", testDeviceSecret, 2))
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("old version accepted: status %d", rr.Code)
	}

	// Version 3 — meets minimum, should allow
	req2 := httptest.NewRequest("POST", "/v1/completions", nil)
	req2.Header.Set(SiderefHeader, makeTokenHeaderWithVersion("devstral-inference", testDeviceSecret, 3))
	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("current version rejected: status %d, body: %s", rr2.Code, rr2.Body.String())
	}
}

func TestMiddleware_HealthBypass(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})

	// /health should pass through without token
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("health check blocked: status %d", rr.Code)
	}
}

func TestMiddleware_RateLimit(t *testing.T) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{
		RateLimitRPS: 2,
	})

	tokenHex := makeTokenHeader("devstral-inference", testDeviceSecret)

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/v1/completions", nil)
		req.Header.Set(SiderefHeader, tokenHex)
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d rejected: status %d", i, rr.Code)
		}
	}

	// 3rd request in same instant should be rate-limited
	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set(SiderefHeader, tokenHex)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
}

func BenchmarkMiddleware_Verify(b *testing.B) {
	mw := SiderefMiddleware(echoHandler, testDeviceSecret, SiderefMiddlewareOpts{})
	tokenHex := makeTokenHeader("devstral-inference", testDeviceSecret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/v1/completions", nil)
		req.Header.Set(SiderefHeader, tokenHex)
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
	}
}
