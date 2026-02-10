package service

import (
	"sync"
	"time"
)

// IngestRateLimiter implements per-mirror rate limiting using a sliding window algorithm.
// It tracks request timestamps for each mirror and enforces a configurable requests-per-minute limit.
type IngestRateLimiter struct {
	mu      sync.Mutex
	windows map[string]*slidingWindow // key: mirrorID
}

// slidingWindow holds the request timestamps for a single mirror.
type slidingWindow struct {
	timestamps []time.Time
	lastClean  time.Time
}

// NewIngestRateLimiter creates a new rate limiter with empty state.
func NewIngestRateLimiter() *IngestRateLimiter {
	return &IngestRateLimiter{
		windows: make(map[string]*slidingWindow),
	}
}

// Allow checks if a request is allowed for the given mirror under its rate limit.
// Returns (true, 0) if allowed, (false, retryAfterSeconds) if rate limit exceeded.
//
// The sliding window is 1 minute. If the number of requests in the current window
// is >= limit, the request is denied and retryAfter indicates when the oldest
// timestamp will expire (allowing the caller to retry safely).
func (r *IngestRateLimiter) Allow(mirrorID string, limit int) (allowed bool, retryAfter int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowDuration := time.Minute
	cutoff := now.Add(-windowDuration)

	// Get or create sliding window for this mirror
	window, exists := r.windows[mirrorID]
	if !exists {
		window = &slidingWindow{
			timestamps: []time.Time{},
			lastClean:  now,
		}
		r.windows[mirrorID] = window
	}

	// Clean expired timestamps (lazy cleanup to avoid memory growth)
	// Only clean if it's been more than 10 seconds since last cleanup
	if now.Sub(window.lastClean) > 10*time.Second {
		filtered := window.timestamps[:0]
		for _, ts := range window.timestamps {
			if ts.After(cutoff) {
				filtered = append(filtered, ts)
			}
		}
		window.timestamps = filtered
		window.lastClean = now
	}

	// Count current requests in window
	currentCount := len(window.timestamps)

	// Check if rate limit exceeded
	if currentCount >= limit {
		// Find oldest timestamp to calculate retry-after
		if len(window.timestamps) > 0 {
			oldestTimestamp := window.timestamps[0]
			expiresAt := oldestTimestamp.Add(windowDuration)
			retryAfterDuration := expiresAt.Sub(now)
			retryAfterSeconds := int(retryAfterDuration.Seconds()) + 1 // Round up

			if retryAfterSeconds < 1 {
				retryAfterSeconds = 1 // Minimum 1 second
			}

			return false, retryAfterSeconds
		}
		// Shouldn't happen, but if no timestamps exist, default to 60 seconds
		return false, 60
	}

	// Allow request and record timestamp
	window.timestamps = append(window.timestamps, now)
	return true, 0
}

// CleanupStale removes windows that have had no activity for 5 minutes.
// This method can be called periodically to prevent unbounded memory growth,
// but is not required for correctness (lazy cleanup in Allow is sufficient).
func (r *IngestRateLimiter) CleanupStale() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	staleThreshold := now.Add(-5 * time.Minute)

	for mirrorID, window := range r.windows {
		// If window has no recent timestamps, remove it
		if len(window.timestamps) == 0 || window.lastClean.Before(staleThreshold) {
			delete(r.windows, mirrorID)
		}
	}
}
