package middleware

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"github.com/gin-gonic/gin"
)

const (
	UserIDKey = "userID"
	UserKey   = "user"
)

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
