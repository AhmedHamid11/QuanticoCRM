package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AuthRateLimiterConfig configures the auth rate limiter
type AuthRateLimiterConfig struct {
	// Max is the maximum number of requests allowed per window (default: 20)
	Max int
	// Window is the time window for rate limiting (default: 1 minute)
	Window time.Duration
}

// DefaultAuthRateLimiterConfig returns the default configuration
// for auth rate limiting: 20 requests per minute per IP
func DefaultAuthRateLimiterConfig() AuthRateLimiterConfig {
	return AuthRateLimiterConfig{
		Max:    20,
		Window: 1 * time.Minute,
	}
}

// rateLimitEntry tracks requests for a single IP
type rateLimitEntry struct {
	mu          sync.Mutex
	count       int
	windowStart time.Time
}

// authRateLimiter is the internal state for the rate limiter
type authRateLimiter struct {
	config  AuthRateLimiterConfig
	entries sync.Map // map[string]*rateLimitEntry
	stopCh  chan struct{}
}

// NewAuthRateLimiter creates a new rate limiter middleware for auth endpoints.
// It limits requests per IP address with configurable max and window settings.
// When the limit is reached, returns 429 with JSON error and Retry-After header.
func NewAuthRateLimiter(config AuthRateLimiterConfig) fiber.Handler {
	// Apply defaults for zero values
	if config.Max <= 0 {
		config.Max = 5
	}
	if config.Window <= 0 {
		config.Window = 1 * time.Minute
	}

	limiter := &authRateLimiter{
		config: config,
		stopCh: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go limiter.cleanup()

	return limiter.handler
}

// getClientIP extracts the client IP, preferring X-Forwarded-For for proxied requests
func getClientIP(c *fiber.Ctx) string {
	// Check X-Forwarded-For first (for requests behind a proxy/load balancer)
	forwarded := c.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}
	return c.IP()
}

// handler is the Fiber middleware handler
func (l *authRateLimiter) handler(c *fiber.Ctx) error {
	ip := getClientIP(c)
	now := time.Now()

	// Get or create entry for this IP
	entryI, _ := l.entries.LoadOrStore(ip, &rateLimitEntry{
		count:       0,
		windowStart: now,
	})
	entry := entryI.(*rateLimitEntry)

	// Lock the entry for thread-safe access
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Check if window has expired
	if now.Sub(entry.windowStart) >= l.config.Window {
		// Reset the window
		entry.count = 0
		entry.windowStart = now
	}

	// Increment counter
	entry.count++

	// Check if limit exceeded
	if entry.count > l.config.Max {
		// Calculate retry-after in seconds
		retryAfter := int(l.config.Window.Seconds())

		// Set Retry-After header
		c.Set("Retry-After", strconv.Itoa(retryAfter))

		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "Too many authentication attempts",
			"message":     "Rate limit exceeded. Please wait before trying again.",
			"retry_after": retryAfter,
		})
	}

	return c.Next()
}

// cleanup runs every 5 minutes to remove expired entries
// This prevents memory leaks from abandoned rate limit entries
func (l *authRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			l.entries.Range(func(key, value interface{}) bool {
				entry := value.(*rateLimitEntry)
				// Remove entries that haven't been accessed in 2 windows
				if now.Sub(entry.windowStart) >= 2*l.config.Window {
					l.entries.Delete(key)
				}
				return true
			})
		case <-l.stopCh:
			return
		}
	}
}
