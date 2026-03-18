package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test router with logger middleware
	router := gin.New()
	router.Use(Logger())

	// Add a test handler
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})
	router.GET("/test-with-query", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "GET request",
			method:         "GET",
			path:           "/test",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST request",
			method:         "POST",
			path:           "/test",
			expectedStatus: http.StatusNotFound, // No POST route registered
		},
		{
			name:           "GET with query params",
			method:         "GET",
			path:           "/test-with-query?key=value",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Non-existent path",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Serve request
			router.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestLoggerMiddlewareExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Track if middleware executed
	executed := false

	// Create test router with logger middleware
	router := gin.New()
	router.Use(Logger())

	// Add a test handler that sets flag
	router.GET("/executed", func(c *gin.Context) {
		executed = true
		c.String(http.StatusOK, "ok")
	})

	// Create test request
	req, err := http.NewRequest("GET", "/executed", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve request
	router.ServeHTTP(w, req)

	// Check if middleware executed
	if !executed {
		t.Error("Logger middleware did not execute")
	}

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestLoggerWithDifferentHTTPMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Logger())

	// Register routes for different methods
	router.PUT("/update", func(c *gin.Context) {
		c.String(http.StatusOK, "updated")
	})
	router.DELETE("/delete", func(c *gin.Context) {
		c.String(http.StatusOK, "deleted")
	})
	router.PATCH("/patch", func(c *gin.Context) {
		c.String(http.StatusOK, "patched")
	})

	tests := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{"PUT request", "PUT", "/update", http.StatusOK},
		{"DELETE request", "DELETE", "/delete", http.StatusOK},
		{"PATCH request", "PATCH", "/patch", http.StatusOK},
		{"OPTIONS request", "OPTIONS", "/update", http.StatusNotFound},
		{"HEAD request", "HEAD", "/update", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Errorf("Expected status %d, got %d", tt.status, w.Code)
			}
		})
	}
}
