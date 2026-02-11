package radius

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 10.0, // 10 req/sec
		BurstSize:         20,   // 20 token burst
		CleanupInterval:   1 * time.Minute,
		MaxAge:            2 * time.Minute,
	}

	rl := NewRateLimiter(config)

	// Test: Burst should allow initial requests
	ip := "192.168.1.100:12345"
	allowed := 0
	for i := 0; i < 20; i++ {
		if rl.Allow(ip) {
			allowed++
		}
	}

	if allowed != 20 {
		t.Errorf("Expected 20 requests in burst, got %d", allowed)
	}

	// Test: 21st request should be denied (burst exhausted)
	if rl.Allow(ip) {
		t.Error("Expected rate limit to block request after burst")
	}
}

func TestRateLimiter_MultipleIPs(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 5.0,
		BurstSize:         10,
		CleanupInterval:   1 * time.Minute,
		MaxAge:            2 * time.Minute,
	}

	rl := NewRateLimiter(config)

	// Test: Different IPs should have independent limits
	ip1 := "10.0.0.1:1234"
	ip2 := "10.0.0.2:5678"

	// Exhaust ip1's burst
	for i := 0; i < 10; i++ {
		if !rl.Allow(ip1) {
			t.Errorf("IP1 request %d should be allowed", i)
		}
	}

	// ip1 should be rate limited now
	if rl.Allow(ip1) {
		t.Error("IP1 should be rate limited after burst")
	}

	// ip2 should still have full burst available
	for i := 0; i < 10; i++ {
		if !rl.Allow(ip2) {
			t.Errorf("IP2 request %d should be allowed", i)
		}
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 10.0, // 10 tokens/sec = 1 token per 100ms
		BurstSize:         5,
		CleanupInterval:   1 * time.Minute,
		MaxAge:            2 * time.Minute,
	}

	rl := NewRateLimiter(config)

	ip := "172.16.0.50:9999"

	// Exhaust burst
	for i := 0; i < 5; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("Initial request %d should be allowed", i)
		}
	}

	// Should be rate limited
	if rl.Allow(ip) {
		t.Error("Should be rate limited after exhausting burst")
	}

	// Wait for tokens to refill (10 req/sec = 100ms per token)
	// Wait 250ms = 2.5 tokens refilled
	time.Sleep(250 * time.Millisecond)

	// Should allow 2 more requests now
	if !rl.Allow(ip) {
		t.Error("Expected request to be allowed after token refill")
	}
	if !rl.Allow(ip) {
		t.Error("Expected second request to be allowed after token refill")
	}

	// Third request should be blocked (only 2 tokens refilled)
	if rl.Allow(ip) {
		t.Error("Expected third request to be blocked")
	}
}

func TestRateLimiter_ExtractIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.1:12345", "192.168.1.1"},
		{"10.0.0.1:8080", "10.0.0.1"},
		{"127.0.0.1:1812", "127.0.0.1"},
		{"[::1]:1812", "::1"},
		{"[2001:db8::1]:1812", "2001:db8::1"},
		{"192.168.1.1", "192.168.1.1"}, // No port
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractIP(tt.input)
			if result != tt.expected {
				t.Errorf("extractIP(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         20,
		CleanupInterval:   100 * time.Millisecond,
		MaxAge:            200 * time.Millisecond,
	}

	rl := NewRateLimiter(config)

	// Create limiters for multiple IPs
	ip1 := "10.0.0.1:1234"
	ip2 := "10.0.0.2:5678"
	ip3 := "10.0.0.3:9999"

	rl.Allow(ip1)
	rl.Allow(ip2)
	rl.Allow(ip3)

	stats := rl.Stats()
	if stats.TrackedIPs != 3 {
		t.Errorf("Expected 3 tracked IPs, got %d", stats.TrackedIPs)
	}

	// Keep ip1 active, let ip2 and ip3 go stale
	time.Sleep(150 * time.Millisecond)
	rl.Allow(ip1) // Refresh ip1

	// Wait for cleanup to run (MaxAge=200ms, we're at 150ms)
	time.Sleep(200 * time.Millisecond)

	// Trigger cleanup manually to ensure it runs
	rl.cleanup()

	stats = rl.Stats()
	// ip1 should remain, ip2 and ip3 should be cleaned up
	if stats.TrackedIPs > 1 {
		t.Errorf("Expected cleanup to remove old IPs, still tracking %d IPs", stats.TrackedIPs)
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 100.0,
		BurstSize:         200,
		CleanupInterval:   1 * time.Minute,
		MaxAge:            2 * time.Minute,
	}

	rl := NewRateLimiter(config)

	// Test concurrent access from multiple goroutines
	var wg sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		ip := fmt.Sprintf("10.0.0.%d:1234", i)

		go func(addr string) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				rl.Allow(addr)
			}
		}(ip)
	}

	wg.Wait()

	stats := rl.Stats()
	if stats.TrackedIPs != numGoroutines {
		t.Errorf("Expected %d tracked IPs, got %d", numGoroutines, stats.TrackedIPs)
	}
}

func TestRateLimiter_DoSProtection(t *testing.T) {
	// Simulate DoS attack: rapid requests from single IP
	config := &RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         20,
		CleanupInterval:   1 * time.Minute,
		MaxAge:            2 * time.Minute,
	}

	rl := NewRateLimiter(config)

	attackerIP := "203.0.113.100:12345"
	allowedCount := 0
	blockedCount := 0

	// Simulate 1000 rapid requests (DoS attack)
	for i := 0; i < 1000; i++ {
		if rl.Allow(attackerIP) {
			allowedCount++
		} else {
			blockedCount++
		}
	}

	// Should allow burst (20) and block the rest
	if allowedCount > 25 {
		t.Errorf("Too many requests allowed during DoS: %d (expected ~20)", allowedCount)
	}

	if blockedCount < 975 {
		t.Errorf("Not enough requests blocked during DoS: %d (expected ~980)", blockedCount)
	}

	t.Logf("DoS protection: allowed %d, blocked %d out of 1000 requests", allowedCount, blockedCount)
}

func TestRateLimiter_Stats(t *testing.T) {
	config := &RateLimitConfig{
		RequestsPerSecond: 15.0,
		BurstSize:         30,
		CleanupInterval:   5 * time.Minute,
		MaxAge:            10 * time.Minute,
	}

	rl := NewRateLimiter(config)

	// Add some IPs
	rl.Allow("10.0.0.1:1234")
	rl.Allow("10.0.0.2:5678")

	stats := rl.Stats()

	if stats.TrackedIPs != 2 {
		t.Errorf("Expected 2 tracked IPs, got %d", stats.TrackedIPs)
	}

	if stats.Config.RequestsPerSecond != 15.0 {
		t.Errorf("Expected RequestsPerSecond=15.0, got %f", stats.Config.RequestsPerSecond)
	}

	if stats.Config.BurstSize != 30 {
		t.Errorf("Expected BurstSize=30, got %d", stats.Config.BurstSize)
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	if config.RequestsPerSecond != 10.0 {
		t.Errorf("Default RequestsPerSecond should be 10.0, got %f", config.RequestsPerSecond)
	}

	if config.BurstSize != 20 {
		t.Errorf("Default BurstSize should be 20, got %d", config.BurstSize)
	}

	if config.CleanupInterval != 5*time.Minute {
		t.Errorf("Default CleanupInterval should be 5m, got %v", config.CleanupInterval)
	}

	if config.MaxAge != 15*time.Minute {
		t.Errorf("Default MaxAge should be 15m, got %v", config.MaxAge)
	}
}
