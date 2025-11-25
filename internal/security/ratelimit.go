package security

import (
	"fmt"
	"sync"
	"time"

	"dbbackup/internal/logger"
)

// RateLimiter tracks connection attempts and enforces rate limiting
type RateLimiter struct {
	attempts      map[string]*attemptTracker
	mu            sync.RWMutex
	maxRetries    int
	baseDelay     time.Duration
	maxDelay      time.Duration
	resetInterval time.Duration
	log           logger.Logger
}

// attemptTracker tracks connection attempts for a specific host
type attemptTracker struct {
	count       int
	lastAttempt time.Time
	nextAllowed time.Time
}

// NewRateLimiter creates a new rate limiter for connection attempts
func NewRateLimiter(maxRetries int, log logger.Logger) *RateLimiter {
	return &RateLimiter{
		attempts:      make(map[string]*attemptTracker),
		maxRetries:    maxRetries,
		baseDelay:     1 * time.Second,
		maxDelay:      60 * time.Second,
		resetInterval: 5 * time.Minute,
		log:           log,
	}
}

// CheckAndWait checks if connection is allowed and waits if rate limited
// Returns error if max retries exceeded
func (rl *RateLimiter) CheckAndWait(host string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	tracker, exists := rl.attempts[host]

	if !exists {
		// First attempt, allow immediately
		rl.attempts[host] = &attemptTracker{
			count:       1,
			lastAttempt: now,
			nextAllowed: now,
		}
		return nil
	}

	// Reset counter if enough time has passed
	if now.Sub(tracker.lastAttempt) > rl.resetInterval {
		rl.log.Debug("Resetting rate limit counter", "host", host)
		tracker.count = 1
		tracker.lastAttempt = now
		tracker.nextAllowed = now
		return nil
	}

	// Check if max retries exceeded
	if tracker.count >= rl.maxRetries {
		return fmt.Errorf("max connection retries (%d) exceeded for host %s, try again in %v",
			rl.maxRetries, host, rl.resetInterval)
	}

	// Calculate exponential backoff delay
	delay := rl.calculateDelay(tracker.count)
	tracker.nextAllowed = tracker.lastAttempt.Add(delay)

	// Wait if necessary
	if now.Before(tracker.nextAllowed) {
		waitTime := tracker.nextAllowed.Sub(now)
		rl.log.Info("Rate limiting connection attempt",
			"host", host,
			"attempt", tracker.count,
			"wait_seconds", int(waitTime.Seconds()))

		rl.mu.Unlock()
		time.Sleep(waitTime)
		rl.mu.Lock()
	}

	// Update tracker
	tracker.count++
	tracker.lastAttempt = time.Now()

	return nil
}

// RecordSuccess resets the attempt counter for successful connections
func (rl *RateLimiter) RecordSuccess(host string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if tracker, exists := rl.attempts[host]; exists {
		rl.log.Debug("Connection successful, resetting rate limit", "host", host)
		tracker.count = 0
		tracker.lastAttempt = time.Now()
		tracker.nextAllowed = time.Now()
	}
}

// RecordFailure increments the failure counter
func (rl *RateLimiter) RecordFailure(host string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	tracker, exists := rl.attempts[host]

	if !exists {
		rl.attempts[host] = &attemptTracker{
			count:       1,
			lastAttempt: now,
			nextAllowed: now.Add(rl.baseDelay),
		}
		return
	}

	tracker.count++
	tracker.lastAttempt = now
	tracker.nextAllowed = now.Add(rl.calculateDelay(tracker.count))

	rl.log.Warn("Connection failed",
		"host", host,
		"attempt", tracker.count,
		"max_retries", rl.maxRetries)
}

// calculateDelay calculates exponential backoff delay
func (rl *RateLimiter) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s, max 60s
	delay := rl.baseDelay * time.Duration(1<<uint(attempt-1))
	if delay > rl.maxDelay {
		delay = rl.maxDelay
	}
	return delay
}

// GetStatus returns current rate limit status for a host
func (rl *RateLimiter) GetStatus(host string) (attempts int, nextAllowed time.Time, isLimited bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	tracker, exists := rl.attempts[host]
	if !exists {
		return 0, time.Now(), false
	}

	now := time.Now()
	isLimited = now.Before(tracker.nextAllowed)

	return tracker.count, tracker.nextAllowed, isLimited
}

// Cleanup removes old entries from rate limiter
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for host, tracker := range rl.attempts {
		if now.Sub(tracker.lastAttempt) > rl.resetInterval*2 {
			delete(rl.attempts, host)
		}
	}
}
