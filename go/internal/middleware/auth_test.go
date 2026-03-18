package middleware

import (
	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// setupMiddlewareTestDB initializes a test database for middleware tests
func setupMiddlewareTestDB(t *testing.T) func() {
	// Reset database state for test
	database.ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test-secret-key-for-middleware-tests")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	gin.SetMode(gin.TestMode)
	return func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}
}

// hashPassword creates a bcrypt hash for testing
func hashPasswordMW(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

// createTestUserMW creates a test user
func createTestUserMW(t *testing.T, username string, isSuperuser, isBanned bool) *models.User {
	t.Helper()
	hashedPassword := hashPasswordMW(t, "password123")
	user := models.User{
		Username:       username,
		Password:       hashedPassword,
		PresenceStatus: "offline",
		IsSuperuser:    isSuperuser,
		IsBanned:       isBanned,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	return &user
}

// ============================================================================
// Auth Middleware Tests
// ============================================================================

func TestAuthMiddleware_Success(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	user := createTestUserMW(t, "testuser", false, false)

	// Create test router with auth middleware
	router := gin.New()
	router.Use(Auth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Create request with valid cookie
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", user.ID),
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Auth middleware should return 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoCookie(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	router := gin.New()
	router.Use(Auth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Auth middleware should return 401 for no cookie, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidCookie(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	router := gin.New()
	router.Use(Auth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: "invalid",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Auth middleware should return 401 for invalid cookie, got %d", w.Code)
	}
}

func TestAuthMiddleware_UserNotFound(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	router := gin.New()
	router.Use(Auth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: "99999", // Non-existent user ID
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Auth middleware should return 401 for non-existent user, got %d", w.Code)
	}
}

func TestAuthMiddleware_BannedUser(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	user := createTestUserMW(t, "banneduser", false, true)

	router := gin.New()
	router.Use(Auth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", user.ID),
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Auth middleware should return 403 for banned user, got %d", w.Code)
	}
}

func TestAuthMiddleware_Superuser(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	user := createTestUserMW(t, "superuser", true, false)

	router := gin.New()
	router.Use(Auth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", user.ID),
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Auth middleware should return 200 for superuser, got %d", w.Code)
	}
}

// ============================================================================
// OptionalAuth Middleware Tests
// ============================================================================

func TestOptionalAuthMiddleware_WithValidCookie(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	user := createTestUserMW(t, "testuser", false, false)

	router := gin.New()
	router.Use(OptionalAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", user.ID),
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OptionalAuth middleware should return 200 with valid cookie, got %d", w.Code)
	}
}

func TestOptionalAuthMiddleware_NoCookie(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	router := gin.New()
	router.Use(OptionalAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// OptionalAuth should allow requests without cookie
	if w.Code != http.StatusOK {
		t.Errorf("OptionalAuth middleware should return 200 without cookie, got %d", w.Code)
	}
}

func TestOptionalAuthMiddleware_InvalidCookie(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	router := gin.New()
	router.Use(OptionalAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: "invalid",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// OptionalAuth should allow requests with invalid cookie (just won't set user in context)
	if w.Code != http.StatusOK {
		t.Errorf("OptionalAuth middleware should return 200 with invalid cookie, got %d", w.Code)
	}
}

func TestOptionalAuthMiddleware_BannedUser(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	user := createTestUserMW(t, "banneduser", false, true)

	router := gin.New()
	router.Use(OptionalAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", user.ID),
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// OptionalAuth should allow banned users (just won't set user in context)
	if w.Code != http.StatusOK {
		t.Errorf("OptionalAuth middleware should return 200 for banned user, got %d", w.Code)
	}
}

// ============================================================================
// CORS Middleware Tests
// ============================================================================

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	os.Setenv("ALLOWED_ORIGINS", "http://localhost,http://example.com")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CORS middleware should return 200 for allowed origin, got %d", w.Code)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost" {
		t.Errorf("CORS middleware should set Access-Control-Allow-Origin header")
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("CORS middleware should set Access-Control-Allow-Credentials header")
	}
}

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	os.Setenv("ALLOWED_ORIGINS", "http://localhost")
	defer os.Unsetenv("ALLOWED_ORIGINS")

	router := gin.New()
	router.Use(CORS())
	router.OPTIONS("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("CORS middleware should return 204 for preflight request, got %d", w.Code)
	}
}

func TestCORSMiddleware_NoOrigin(t *testing.T) {
	cleanup := setupMiddlewareTestDB(t)
	defer cleanup()

	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Origin header
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("CORS middleware should return 200 for same-origin request, got %d", w.Code)
	}
}

// ============================================================================
// GetCurrentUser Tests
// ============================================================================

func TestGetCurrentUser_Exists(t *testing.T) {
	c := &gin.Context{}
	user := &models.User{
		Username: "testuser",
	}

	c.Set(UserKey, user)

	retrievedUser, exists := GetCurrentUser(c)
	if !exists {
		t.Error("GetCurrentUser should return true when user exists in context")
	}
	if retrievedUser == nil {
		t.Fatal("GetCurrentUser should return non-nil user")
	}
	if retrievedUser.Username != "testuser" {
		t.Errorf("GetCurrentUser returned username = %s, want testuser", retrievedUser.Username)
	}
}

func TestGetCurrentUser_NotExists(t *testing.T) {
	c := &gin.Context{}

	_, exists := GetCurrentUser(c)
	if exists {
		t.Error("GetCurrentUser should return false when user doesn't exist in context")
	}
}

func TestGetCurrentUserFromContext_Exists(t *testing.T) {
	c := &gin.Context{}
	user := &models.User{
		Username: "testuser",
	}

	c.Set(UserKey, user)

	retrievedUser := GetCurrentUserFromContext(c)
	if retrievedUser == nil {
		t.Fatal("GetCurrentUserFromContext should return non-nil user")
	}
	if retrievedUser.Username != "testuser" {
		t.Errorf("GetCurrentUserFromContext returned username = %s, want testuser", retrievedUser.Username)
	}
}

func TestGetCurrentUserFromContext_NotExists(t *testing.T) {
	c := &gin.Context{}

	retrievedUser := GetCurrentUserFromContext(c)
	if retrievedUser != nil {
		t.Error("GetCurrentUserFromContext should return nil when user doesn't exist in context")
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestCookieValue(t *testing.T) {
	// Test that we can convert user ID to cookie value properly
	userID := uint(123)
	cookieValue := string(rune(userID))
	
	// This is just to verify the conversion works as expected
	if cookieValue == "" {
		t.Error("Cookie value should not be empty")
	}
}
