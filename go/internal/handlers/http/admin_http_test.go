package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/testutil"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Test Setup Helpers for Admin HTTP
// ============================================================================

func setupAdminHTTPTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	cfg, dbCleanup := testutil.SetupTestDB(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	cfg, _ = config.Load()
	authHandler := NewAuthHandler(cfg)
	apiHandler := NewAPIHandler(cfg)

	authHandler.RegisterRoutes(router)
	apiHandler.RegisterRoutes(router)
	apiHandler.RegisterAdminRoutes(router)

	return cfg, router, dbCleanup
}

func createSuperuserForAdmin(t *testing.T) *models.User {
	t.Helper()
	return createTestUserForAPI(t, "adminsuperuser", true)
}

// ============================================================================
// GetBannedUsers Tests
// ============================================================================

func TestGetBannedUsers_Success(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	superuser := createSuperuserForAdmin(t)
	
	// Create banned users
	bannedUser1 := createTestUserForAPI(t, "banneduser1", false)
	bannedUser1.IsBanned = true
	bannedUser1.BanReason = "Test ban 1"
	database.DB.Save(bannedUser1)

	bannedUser2 := createTestUserForAPI(t, "banneduser2", false)
	bannedUser2.IsBanned = true
	bannedUser2.BanReason = "Test ban 2"
	database.DB.Save(bannedUser2)

	req, _ := http.NewRequest("GET", "/api/v1/admin/banned_users", nil)
	addAuthCookieForAPI(req, superuser.ID, superuser.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array in response")
	}

	if len(users) < 2 {
		t.Errorf("Expected at least 2 banned users, got %d", len(users))
	}
}

func TestGetBannedUsers_NotSuperuser(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notsuperuser", false)

	req, _ := http.NewRequest("GET", "/api/v1/admin/banned_users", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestGetBannedUsers_Unauthenticated(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/admin/banned_users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetBannedUsers_Empty(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	superuser := createSuperuserForAdmin(t)

	req, _ := http.NewRequest("GET", "/api/v1/admin/banned_users", nil)
	addAuthCookieForAPI(req, superuser.ID, superuser.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array")
	}

	if len(users) != 0 {
		t.Errorf("Expected 0 banned users, got %d", len(users))
	}
}

// ============================================================================
// GetBannedIPs Tests
// ============================================================================

func TestGetBannedIPs_Success(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	superuser := createSuperuserForAdmin(t)
	
	// Create banned user with IPs
	bannedUser := createTestUserForAPI(t, "bannedipuser", false)
	bannedUser.IsBanned = true
	bannedUser.BanReason = "IP ban"
	bannedUser.BannedIPs = "192.168.1.1,192.168.1.2"
	now := time.Now()
	bannedUser.BannedAt = &now
	database.DB.Save(bannedUser)

	req, _ := http.NewRequest("GET", "/api/v1/admin/banned_ips", nil)
	addAuthCookieForAPI(req, superuser.ID, superuser.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}

	bannedIPs, ok := response["banned_ips"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected banned_ips object in response")
	}

	if len(bannedIPs) < 2 {
		t.Errorf("Expected at least 2 banned IPs, got %d", len(bannedIPs))
	}
}

func TestGetBannedIPs_NoBannedIPs(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	superuser := createSuperuserForAdmin(t)

	req, _ := http.NewRequest("GET", "/api/v1/admin/banned_ips", nil)
	addAuthCookieForAPI(req, superuser.ID, superuser.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}

	bannedIPs, ok := response["banned_ips"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected banned_ips object")
	}

	if len(bannedIPs) != 0 {
		t.Errorf("Expected 0 banned IPs, got %d", len(bannedIPs))
	}
}

func TestGetBannedIPs_NotSuperuser(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notsuperuserips", false)

	req, _ := http.NewRequest("GET", "/api/v1/admin/banned_ips", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// ChangeOwnPassword Tests
// ============================================================================

func TestChangeOwnPassword_Success(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "changepassworduser", false)

	passwordData := map[string]string{
		"current_password": "password123",
		"new_password":     "newpassword123",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", "/api/v1/user/change_password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}
}

func TestChangeOwnPassword_WrongCurrentPassword(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "wrongcurrentpassworduser", false)

	passwordData := map[string]string{
		"current_password": "wrongpassword",
		"new_password":     "newpassword123",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", "/api/v1/user/change_password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestChangeOwnPassword_ShortNewPassword(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "shortnewpassworduser", false)

	passwordData := map[string]string{
		"current_password": "password123",
		"new_password":     "short",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", "/api/v1/user/change_password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestChangeOwnPassword_Unauthenticated(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	passwordData := map[string]string{
		"current_password": "password123",
		"new_password":     "newpassword123",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", "/api/v1/user/change_password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

// ============================================================================
// AdminChangePassword Tests (admin changing user's password)
// ============================================================================

func TestAdminChangePassword_Success(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	superuser := createSuperuserForAdmin(t)
	targetUser := createTestUserForAPI(t, "targetadminpassuser", false)

	passwordData := map[string]string{
		"new_password": "adminsetpassword123",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/user/%d/change_password", targetUser.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, superuser.ID, superuser.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}
}

func TestAdminChangePassword_NotSuperuser(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notsuperadminuser", false)
	targetUser := createTestUserForAPI(t, "targetnotsuperadminuser", false)

	passwordData := map[string]string{
		"new_password": "adminsetpassword123",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/user/%d/change_password", targetUser.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestAdminChangePassword_UserNotFound(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	superuser := createSuperuserForAdmin(t)

	passwordData := map[string]string{
		"new_password": "adminsetpassword123",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", "/api/v1/admin/user/999/change_password", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, superuser.ID, superuser.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAdminChangePassword_ShortPassword(t *testing.T) {
	_, router, cleanup := setupAdminHTTPTestDB(t)
	defer cleanup()

	superuser := createSuperuserForAdmin(t)
	targetUser := createTestUserForAPI(t, "shortadminpassuser", false)

	passwordData := map[string]string{
		"new_password": "short",
	}
	jsonData, _ := json.Marshal(passwordData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/user/%d/change_password", targetUser.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, superuser.ID, superuser.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
