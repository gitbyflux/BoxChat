package http

import (
	"boxchat/internal/config"
	"boxchat/internal/testutil"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupTestRouter creates a test router with all routes registered
func setupTestRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	authHandler := NewAuthHandler(cfg)
	apiHandler := NewAPIHandler(cfg)

	authHandler.RegisterRoutes(r)
	apiHandler.RegisterRoutes(r)

	return r
}

// ============================================================================
// Health Check Tests
// ============================================================================

func TestHealthCheck(t *testing.T) {
	cfg, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	router := setupTestRouter(cfg)

	// Test GET /api/v1/reactions (public endpoint)
	req, _ := http.NewRequest("GET", "/api/v1/reactions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 200
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// ============================================================================
// Authentication Tests
// ============================================================================

func TestLoginInvalidCredentials(t *testing.T) {
	cfg, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	router := setupTestRouter(cfg)

	// Test login with wrong password
	loginData := map[string]string{
		"username": "nonexistent",
		"password": "wrongpassword",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestSessionCheckInvalid(t *testing.T) {
	cfg, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	router := setupTestRouter(cfg)

	// Test session check without cookie
	req, _ := http.NewRequest("GET", "/api/v1/auth/session", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("Expected status 401 for invalid session, got %d", w.Code)
	}
}

// ============================================================================
// User API Tests
// ============================================================================

func TestGetReactionsPublic(t *testing.T) {
	cfg, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	router := setupTestRouter(cfg)

	// Test get reactions (public endpoint)
	req, _ := http.NewRequest("GET", "/api/v1/reactions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Just verify we get a valid JSON response
	var response interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Expected valid JSON response: %v", err)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================
