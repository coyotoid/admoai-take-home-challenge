package adspots

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type TokenBucket struct {
	mu sync.Mutex

	capacity   int       // Maximum number of tokens
	tokens     int       // Current number of tokens
	refillRate int       // Tokens added per second
	lastRefill time.Time // Last time tokens were refilled
}

type RateLimiter struct {
	mu sync.RWMutex

	buckets    map[string]*TokenBucket
	capacity   int           // Max tokens per bucket
	refillRate int           // Tokens per second
	cleanup    time.Duration // How often to clean up expired buckets
}

type RateLimiterConfig struct {
	RequestsPerSecond int           // Number of requests allowed per second
	BurstSize         int           // Maximum burst size (token bucket capacity)
	CleanupInterval   time.Duration // How often to clean up expired entries
}

func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	if config.RequestsPerSecond <= 0 {
		config.RequestsPerSecond = 10 // Default: 10 requests per second
	}
	if config.BurstSize <= 0 {
		config.BurstSize = config.RequestsPerSecond * 2 // Default: 2x the rate
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute // Default: cleanup every 5 minutes
	}

	rl := &RateLimiter{
		buckets:    make(map[string]*TokenBucket),
		capacity:   config.BurstSize,
		refillRate: config.RequestsPerSecond,
		cleanup:    config.CleanupInterval,
	}

	go rl.startCleanup()

	return rl
}

func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	tokensToAdd := int(elapsed * float64(tb.refillRate))
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) GetBucket(clientID string) *TokenBucket {
	rl.mu.RLock()
	bucket, exists := rl.buckets[clientID]
	rl.mu.RUnlock()

	if exists {
		return bucket
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bucket, exists := rl.buckets[clientID]; exists {
		return bucket
	}

	bucket = NewTokenBucket(rl.capacity, rl.refillRate)
	rl.buckets[clientID] = bucket
	return bucket
}

func (rl *RateLimiter) Allow(clientID string) bool {
	bucket := rl.GetBucket(clientID)
	return bucket.Allow()
}

func (rl *RateLimiter) startCleanup() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanupOldBuckets()
	}
}

func (rl *RateLimiter) cleanupOldBuckets() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.cleanup)
	for clientID, bucket := range rl.buckets {
		bucket.mu.Lock()
		if bucket.lastRefill.Before(cutoff) {
			delete(rl.buckets, clientID)
		}
		bucket.mu.Unlock()
	}
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := net.ParseIP(xff); ip != nil {
			return ip.String()
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (rl *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)

		if !rl.Allow(clientIP) {
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.refillRate))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", "1") // Suggest retry after 1 second

			JSONError(w, map[string]string{
				"what":    "Rate limit exceeded",
				"message": "Too many requests. Please try again later.",
			}, http.StatusTooManyRequests)
			return
		}

		bucket := rl.GetBucket(clientIP)
		bucket.mu.Lock()
		remaining := bucket.tokens
		bucket.mu.Unlock()

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.refillRate))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) RateLimitHandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	middleware := rl.RateLimitMiddleware(handler)
	return middleware.ServeHTTP
}
