package radius

import (
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-IP rate limiting for RADIUS requests.
// This protects against DoS attacks by limiting the number of authentication
// attempts from a single IP address within a time window.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter

	// Rate limit configuration
	requestsPerSecond float64
	burstSize         int

	// Cleanup configuration
	cleanupInterval time.Duration
	maxAge          time.Duration
	lastSeen        map[string]time.Time
}

// RateLimitConfig specifies rate limiting parameters.
type RateLimitConfig struct {
	// RequestsPerSecond is the sustained rate allowed per IP
	RequestsPerSecond float64

	// BurstSize is the maximum burst allowed (tokens in bucket)
	BurstSize int

	// CleanupInterval is how often to clean up old limiters
	CleanupInterval time.Duration

	// MaxAge is how long to keep inactive limiters in memory
	MaxAge time.Duration
}

// DefaultRateLimitConfig returns reasonable default rate limit settings.
// Default: 10 requests/second sustained, 20 request burst
// This allows legitimate users while blocking rapid brute-force attempts.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         20,
		CleanupInterval:   5 * time.Minute,
		MaxAge:            15 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		limiters:          make(map[string]*rate.Limiter),
		lastSeen:          make(map[string]time.Time),
		requestsPerSecond: config.RequestsPerSecond,
		burstSize:         config.BurstSize,
		cleanupInterval:   config.CleanupInterval,
		maxAge:            config.MaxAge,
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given IP address should be allowed.
// Returns true if the request is within rate limits, false if it should be rejected.
func (rl *RateLimiter) Allow(remoteAddr string) bool {
	// Extract IP from address (strip port)
	ip := extractIP(remoteAddr)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Get or create limiter for this IP
	limiter, exists := rl.limiters[ip]
	if !exists {
		// Create new limiter for this IP
		limiter = rate.NewLimiter(rate.Limit(rl.requestsPerSecond), rl.burstSize)
		rl.limiters[ip] = limiter
	}

	// Update last seen time
	rl.lastSeen[ip] = time.Now()

	// Check if request is allowed
	return limiter.Allow()
}

// extractIP extracts the IP address from a remote address string.
// Handles both "IP:port" and "IP" formats.
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// No port in address, return as-is
		return remoteAddr
	}
	return host
}

// cleanupLoop periodically removes old limiters from memory.
// This prevents memory leaks from IPs that haven't been seen recently.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes limiters for IPs that haven't been seen recently.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	removed := 0

	for ip, lastSeen := range rl.lastSeen {
		if now.Sub(lastSeen) > rl.maxAge {
			delete(rl.limiters, ip)
			delete(rl.lastSeen, ip)
			removed++
		}
	}

	if removed > 0 {
		// Optional: log cleanup activity (commented to avoid log spam)
		// log.Printf("[ratelimit] cleaned up %d inactive IP limiters", removed)
	}
}

// Stats returns current rate limiter statistics.
func (rl *RateLimiter) Stats() RateLimitStats {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return RateLimitStats{
		TrackedIPs: len(rl.limiters),
		Config: RateLimitConfig{
			RequestsPerSecond: rl.requestsPerSecond,
			BurstSize:         rl.burstSize,
			CleanupInterval:   rl.cleanupInterval,
			MaxAge:            rl.maxAge,
		},
	}
}

// RateLimitStats contains statistics about the rate limiter.
type RateLimitStats struct {
	TrackedIPs int
	Config     RateLimitConfig
}
