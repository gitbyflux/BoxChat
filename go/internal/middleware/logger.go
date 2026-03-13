package middleware

import (
	"time"
	"github.com/gin-gonic/gin"
)

// Logger returns a middleware that logs request details
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		
		c.Next()
		
		latency := time.Since(start)
		
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		
		if query != "" {
			path = path + "?" + query
		}
		
		println("[HTTP]", method, path, statusCode, latency.String(), clientIP)
	}
}
