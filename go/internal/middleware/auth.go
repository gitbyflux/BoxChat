package middleware

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	UserIDKey = "userID"
	UserKey   = "user"
)

// SecurityHeaders returns a middleware that sets security-related HTTP headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking attacks
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		
		// Prevent MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		
		// Enable XSS filter in browsers
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		
		// Content Security Policy - restrict resource loading
		c.Writer.Header().Set("Content-Security-Policy", 
			"default-src 'self'; "+
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data: https:; "+
			"font-src 'self' data:; "+
			"connect-src 'self' ws: wss:; "+
			"frame-ancestors 'none'")
		
		// Referrer Policy - limit referrer information
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy - disable unnecessary features
		c.Writer.Header().Set("Permissions-Policy",
			"geolocation=(), microphone=(), camera=(), payment=(), usb=()")
		
		// Remove Server header (Gin adds it by default)
		c.Writer.Header().Del("Server")
		
		// Cache control for sensitive pages
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Writer.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Writer.Header().Set("Pragma", "no-cache")
			c.Writer.Header().Set("Expires", "0")
		}
		
		c.Next()
	}
}

// CSRF middleware for token-based CSRF protection
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF for safe methods
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}
		
		// Get token from header
		token := c.GetHeader("X-CSRF-Token")
		
		// Get token from cookie
		cookieToken, err := c.Cookie("csrf_token")
		if err != nil {
			// Generate new CSRF token
			token = generateCSRFToken()
			c.SetCookie("csrf_token", token, int(24*time.Hour), "/", "", false, true)
			c.Next()
			return
		}
		
		// Validate token
		if token != cookieToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Invalid CSRF token",
			})
			return
		}
		
		c.Next()
	}
}

// generateCSRFToken generates a random CSRF token
func generateCSRFToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("[CSRF] Failed to generate token: %v", err)
		return ""
	}
	return hex.EncodeToString(bytes)
}

// Recovery returns a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[RECOVERY] Panic recovered: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// Auth returns a middleware that checks if user is authenticated
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get user ID from cookie
		userIDStr, err := c.Cookie("boxchat_uid")
		if err != nil || userIDStr == "" {
			if c.Request.URL.Path != "/api/v1/auth/session" {
				log.Printf("[AUTH] ✗ Unauthorized request to %s", c.Request.URL.Path)
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			log.Printf("[AUTH] ✗ Invalid user ID: %s", userIDStr)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}

		// Get user from database
		var user models.User
		if err := database.DB.First(&user, userID).Error; err != nil {
			log.Printf("[AUTH] ✗ User %d not found in database", userID)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}

		// Check if user is banned
		if user.IsBanned {
			log.Printf("[AUTH] ✗ User %d (%s) is banned", userID, user.Username)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
			})
			return
		}

		// Set user in context
		c.Set(UserIDKey, uint(userID))
		c.Set(UserKey, &user)
	}
}

// OptionalAuth returns a middleware that optionally authenticates user
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, err := c.Cookie("boxchat_uid")
		if err != nil || userIDStr == "" {
			c.Next()
			return
		}
		
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.Next()
			return
		}
		
		var user models.User
		if err := database.DB.First(&user, userID).Error; err == nil && !user.IsBanned {
			c.Set(UserIDKey, uint(userID))
			c.Set(UserKey, &user)
		}
		
		c.Next()
	}
}

// getAllowedOrigins returns list of allowed CORS origins
func getAllowedOrigins() []string {
	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")
	if allowedOriginsEnv == "" {
		// Default: allow localhost and 127.0.0.1
		return []string{"http://localhost", "http://127.0.0.1", "http://localhost:5000", "http://127.0.0.1:5000"}
	}
	return strings.Split(allowedOriginsEnv, ",")
}

// CORS returns a middleware that handles CORS
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOrigins := getAllowedOrigins()

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if strings.HasPrefix(origin, strings.TrimSpace(allowedOrigin)) {
				allowed = true
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		// For requests without origin (same-origin like fetch from same domain), allow
		if origin == "" {
			allowed = true
			// Don't set Access-Control-Allow-Origin for same-origin requests
			// Browser doesn't need CORS headers for same-origin
		}

		// If origin is not allowed and not empty, still allow but don't set CORS headers
		if !allowed && origin != "" {
			c.Next()
			return
		}

		// Always allow credentials for cookies
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
		// Expose Set-Cookie header so client can see cookies being set
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Set-Cookie")
		// Add Vary header for proper caching
		c.Writer.Header().Set("Vary", "Origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// GetCurrentUser returns the current user from context
func GetCurrentUser(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get(UserKey)
	if !exists {
		return nil, false
	}
	u, ok := user.(*models.User)
	return u, ok
}

// GetCurrentUserFromContext returns the current user from context (for WebSocket)
func GetCurrentUserFromContext(c *gin.Context) *models.User {
	if val, exists := c.Get(UserKey); exists {
		if user, ok := val.(*models.User); ok {
			return user
		}
	}
	return nil
}
