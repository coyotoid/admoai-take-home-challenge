package t

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adspots "github.com/coyotoid/admoai-take-home-challenge"
)

func TestTokenBucket_Allow(t *testing.T) {
	bucket := adspots.NewTokenBucket(3, 1)

	for i := 0; i < 3; i++ {
		if !bucket.Allow() {
			t.Errorf("Expected request %d to be allowed", i+1)
		}
	}

	// 4th request should be denied (bucket empty)
	if bucket.Allow() {
		t.Error("Expected 4th request to be denied")
	}

	time.Sleep(1100 * time.Millisecond)

	if !bucket.Allow() {
		t.Error("Expected request to be allowed after refill")
	}

	if bucket.Allow() {
		t.Error("Expected request to be denied after consuming refilled token")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	config := adspots.RateLimiterConfig{
		RequestsPerSecond: 2,
		BurstSize:         2,
		CleanupInterval:   1 * time.Minute,
	}
	rl := adspots.NewRateLimiter(config)

	clientID := "192.168.1.1"

	for i := 0; i < 2; i++ {
		if !rl.Allow(clientID) {
			t.Errorf("Expected request %d to be allowed", i+1)
		}
	}

	if rl.Allow(clientID) {
		t.Error("Expected 3rd request to be denied")
	}

	time.Sleep(600 * time.Millisecond)

	if !rl.Allow(clientID) {
		t.Error("Expected request to be allowed after partial refill")
	}

	if rl.Allow(clientID) {
		t.Error("Expected request to be denied after consuming refilled token")
	}
}

func TestRateLimiter_DifferentClients(t *testing.T) {
	config := adspots.RateLimiterConfig{
		RequestsPerSecond: 1,
		BurstSize:         2,
		CleanupInterval:   1 * time.Minute,
	}
	rl := adspots.NewRateLimiter(config)

	client1 := "192.168.1.1"
	client2 := "192.168.1.2"

	st1 := rl.Allow(client1)
	st2 := rl.Allow(client1)

	if !st1 || !st2 {
		t.Error("Expected client1 to be allowed 2 requests")
	}
	if rl.Allow(client1) {
		t.Error("Expected client1 3rd request to be denied")
	}

	st1 = rl.Allow(client2)
	st2 = rl.Allow(client2)
	if !st1 || !st2 {
		t.Error("Expected client2 to be allowed 2 requests")
	}
	if rl.Allow(client2) {
		t.Error("Expected client2 3rd request to be denied")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	config := adspots.RateLimiterConfig{
		RequestsPerSecond: 2,
		BurstSize:         2,
		CleanupInterval:   1 * time.Minute,
	}
	rl := adspots.NewRateLimiter(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	rateLimitedHandler := rl.RateLimitMiddleware(handler)

	for i := range 2 {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		rateLimitedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for request %d, got %d", i+1, w.Code)
		}

		if limit := w.Header().Get("X-RateLimit-Limit"); limit != "2" {
			t.Errorf("Expected X-RateLimit-Limit to be '2', got '%s'", limit)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	rateLimitedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 for rate limited request, got %d", w.Code)
	}

	if remaining := w.Header().Get("X-RateLimit-Remaining"); remaining != "0" {
		t.Errorf("Expected X-RateLimit-Remaining to be '0', got '%s'", remaining)
	}
	if retryAfter := w.Header().Get("Retry-After"); retryAfter != "1" {
		t.Errorf("Expected Retry-After to be '1', got '%s'", retryAfter)
	}
}
