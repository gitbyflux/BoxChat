package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiterConfig holds configuration for rate limiting
type RateLimiterConfig struct {
	RequestsPerSecond float64
	BurstSize         int
	KeyFunc           func(*gin.Context) string
}

// visitor holds a rate limiter and last seen time
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter middleware for rate limiting
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	config   RateLimiterConfig
}

// NewRateLimiter creates a new rate limiter middleware
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	limiter := &RateLimiter{
		visitors: make(map[string]*visitor),
		config:   config,
	}

	// Start cleanup goroutine
	go limiter.cleanupVisitors()

	return limiter
}

// cleanupVisitors removes old visitors every minute
func (rl *RateLimiter) cleanupVisitors() {
	for range time.Tick(time.Minute) {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getLimiter returns or creates a rate limiter for a key
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize)
		rl.visitors[key] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// Middleware returns the gin middleware function
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.config.KeyFunc(c)
		if key == "" {
			key = c.ClientIP() // Default to IP
		}

		limiter := rl.getLimiter(key)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

// Default rate limiters for different endpoints
var (
	// AuthRateLimiter: 5 requests per second, burst of 10 (for login/register)
	AuthRateLimiter = NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 5,
		BurstSize:         10,
		KeyFunc: func(c *gin.Context) string {
			return c.ClientIP()
		},
	})

	// APIRateLimiter: 10 requests per second, burst of 20 (for general API)
	APIRateLimiter = NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         20,
		KeyFunc: func(c *gin.Context) string {
			return c.ClientIP()
		},
	})

	// StrictRateLimiter: 2 requests per second, burst of 5 (for sensitive operations)
	StrictRateLimiter = NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 2,
		BurstSize:         5,
		KeyFunc: func(c *gin.Context) string {
			// Use user ID if available, otherwise IP
			userID, exists := c.Get("userID")
			if exists {
				return strconv.FormatUint(uint64(userID.(uint)), 10)
			}
			return c.ClientIP()
		},
	})
)
