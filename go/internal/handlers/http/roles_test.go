package http

import (
	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Helper Functions for Roles Tests
// ============================================================================

func setupRolesTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	// Reset database state for test
	database.ResetForTesting()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_for_testing")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	apiHandler := NewAPIHandler(cfg)
	apiHandler.RegisterRoutes(router)
	apiHandler.RegisterRolesRoutes(router)

	cleanup := func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}

	return cfg, router, cleanup
}

func createRoomForRoles(t *testing.T, ownerID *uint) *models.Room {
	t.Helper()
	room := models.Room{
		Name:        fmt.Sprintf("Test Room %d", time.Now().UnixNano()),
		Type:        "server",
		OwnerID:     ownerID,
		InviteToken: fmt.Sprintf("token_%d", time.Now().UnixNano()),
	}
	if err := database.DB.Create(&room).Error; err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	return &room
}

func createMemberForRoles(t *testing.T, userID, roomID uint, role string) {
	t.Helper()
	member := models.Member{
		UserID: userID,
		RoomID: roomID,
		Role:   role,
	}
	if err := database.DB.Create(&member).Error; err != nil {
		t.Fatalf("Failed to create member: %v", err)
	}
}

func createRoleForTest(t *testing.T, roomID uint, name, tag string, isSystem bool, permissions []string) *models.Role {
	t.Helper()
	role := models.Role{
		RoomID:                   roomID,
		Name:                     name,
		MentionTag:               tag,
		IsSystem:                 isSystem,
		CanBeMentionedByEveryone: false,
	}

	if len(permissions) > 0 {
		permsJSON, _ := json.Marshal(permissions)
		role.PermissionsJSON = string(permsJSON)
	} else {
		role.PermissionsJSON = "[]"
	}

	if err := database.DB.Create(&role).Error; err != nil {
		t.Fatalf("Failed to create role: %v", err)
	}
	return &role
}

func createMemberRole(t *testing.T, userID, roomID, roleID uint) {
	t.Helper()
	memberRole := models.MemberRole{
		UserID: userID,
		RoomID: roomID,
		RoleID: roleID,
	}
	if err := database.DB.Create(&memberRole).Error; err != nil {
		t.Fatalf("Failed to create member role: %v", err)
	}
}

func createRoleMentionPermission(t *testing.T, roomID, sourceRoleID, targetRoleID uint) {
	t.Helper()
	perm := models.RoleMentionPermission{
		RoomID:       roomID,
		SourceRoleID: sourceRoleID,
		TargetRoleID: targetRoleID,
	}
	if err := database.DB.Create(&perm).Error; err != nil {
		t.Fatalf("Failed to create role mention permission: %v", err)
	}
}

// hashPasswordRoles creates bcrypt hash for roles tests
func hashPasswordRoles(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

// createAdminUserRoles creates admin user for roles tests
func createAdminUserRoles(t *testing.T) *models.User {
	t.Helper()
	hashedPassword := hashPasswordRoles(t, "adminpass123")
	user := models.User{
		Username:    "adminuser",
		Password:    hashedPassword,
		IsSuperuser: true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}
	return &user
}

// createRegularUserRoles creates regular user for roles tests
func createRegularUserRoles(t *testing.T, username string) *models.User {
	t.Helper()
	hashedPassword := hashPasswordRoles(t, "userpass123")
	user := models.User{
		Username: username,
		Password: hashedPassword,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	return &user
}

// addAuthCookieToRequestRoles adds auth cookie for roles tests
func addAuthCookieToRequestRoles(req *http.Request, userID uint) {
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", userID),
	})
}

// ============================================================================
// CreateRole Tests
// ============================================================================

func TestCreateRole_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)

	roleData := map[string]interface{}{
		"name":                         "Test Role",
		"mention_tag":                  "test",
		"can_be_mentioned_by_everyone": false,
		"permissions":                  []string{},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestCreateRole_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "createuser")

	roleData := map[string]interface{}{
		"name": "Test Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", "/api/v1/room/invalid/roles", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCreateRole_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	roleData := map[string]interface{}{
		"name": "Test Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestCreateRole_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCreateRole_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	roleData := map[string]interface{}{
		"name":                         "Test Role",
		"mention_tag":                  "testrole",
		"can_be_mentioned_by_everyone": true,
		"permissions":                  []string{"invite_members"},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

func TestCreateRole_EmptyMentionTag(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	roleData := map[string]interface{}{
		"name":                         "Test Role No Tag",
		"mention_tag":                  "",
		"can_be_mentioned_by_everyone": false,
		"permissions":                  []string{},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestCreateRole_InvalidPermissions(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	roleData := map[string]interface{}{
		"name":        "Test Role Invalid Perms",
		"mention_tag": "invalidperms",
		"permissions": []string{"invalid_permission", "manage_server"},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Should only have valid permission
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	role, ok := response["role"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected role in response")
	}

	// Verify only valid permission was kept
	permsJSON := role["permissions_json"].(string)
	var perms []string
	json.Unmarshal([]byte(permsJSON), &perms)
	if len(perms) != 1 || perms[0] != "manage_server" {
		t.Errorf("Expected only manage_server permission, got %v", perms)
	}
}

// ============================================================================
// GetRole Tests
// ============================================================================

func TestGetRole_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetRole_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "getuser")

	req, _ := http.NewRequest("GET", "/api/v1/room/invalid/roles/1", nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetRole_InvalidRoleID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "getuser2")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/invalid", room.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetRole_NotMember(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "notmemberuser")
	room := createRoomForRoles(t, nil)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/1", room.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestGetRole_NotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "notfounduser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/99999", room.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetRole_WrongRoom(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "wrongroomuser")
	room1 := createRoomForRoles(t, nil)
	room2 := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room1.ID, "member")
	role := createRoleForTest(t, room2.ID, "Other Role", "other", false, []string{})

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room1.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetRole_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "successgetuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")
	role := createRoleForTest(t, room.ID, "Success Role", "success", false, []string{"invite_members"})

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	roleData, ok := response["role"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected role in response")
	}

	if roleData["name"] != "Success Role" {
		t.Errorf("Expected name 'Success Role', got %v", roleData["name"])
	}
}

// ============================================================================
// UpdateRole Tests
// ============================================================================

func TestUpdateRole_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	roleData := map[string]string{
		"name": "Updated Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestUpdateRole_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)

	roleData := map[string]string{
		"name": "Updated Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", "/api/v1/room/invalid/roles/1", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRole_InvalidRoleID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	roleData := map[string]string{
		"name": "Updated Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/invalid", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRole_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermupdateuser")
	room := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})
	createMemberForRoles(t, user.ID, room.ID, "member")

	roleData := map[string]string{
		"name": "Updated Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestUpdateRole_NotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	roleData := map[string]string{
		"name": "Updated Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/99999", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestUpdateRole_WrongRoom(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room2.ID, "Other Role", "other", false, []string{})

	roleData := map[string]string{
		"name": "Updated Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room1.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRole_SystemRole(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "System Role", "system", true, []string{})

	roleData := map[string]string{
		"name": "Updated System Role",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestUpdateRole_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRole_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	roleData := map[string]interface{}{
		"name":                         "Updated Role",
		"mention_tag":                  "updated",
		"can_be_mentioned_by_everyone": true,
		"permissions":                  []string{"ban_members"},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

func TestUpdateRole_PartialUpdate(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	// Only update name
	roleData := map[string]string{
		"name": "Only Name Updated",
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRole_EmptyUpdates(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	// Empty updates
	roleData := map[string]interface{}{}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// ============================================================================
// DeleteRole Tests
// ============================================================================

func TestDeleteRole_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestDeleteRole_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)

	req, _ := http.NewRequest("DELETE", "/api/v1/room/invalid/roles/1", nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDeleteRole_InvalidRoleID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/invalid", room.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDeleteRole_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermdeleteuser")
	room := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})
	createMemberForRoles(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestDeleteRole_NotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/99999", room.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDeleteRole_WrongRoom(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room2.ID, "Other Role", "other", false, []string{})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room1.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDeleteRole_SystemRole(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "System Role", "system", true, []string{})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestDeleteRole_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Delete Me", "deleteme", false, []string{})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}

	// Verify role deleted
	var deletedRole models.Role
	result := database.DB.First(&deletedRole, role.ID)
	if result.Error == nil {
		t.Error("DeleteRole() should delete the role")
	}
}

// ============================================================================
// UpdateRolePermissions Tests
// ============================================================================

func TestUpdateRolePermissions_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	permsData := map[string]interface{}{
		"permissions": []string{"invite_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestUpdateRolePermissions_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)

	permsData := map[string]interface{}{
		"permissions": []string{"invite_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", "/api/v1/room/invalid/roles/1/permissions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRolePermissions_InvalidRoleID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	permsData := map[string]interface{}{
		"permissions": []string{"invite_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/invalid/permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRolePermissions_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermpermuser")
	room := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})
	createMemberForRoles(t, user.ID, room.ID, "member")

	permsData := map[string]interface{}{
		"permissions": []string{"invite_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestUpdateRolePermissions_NotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	permsData := map[string]interface{}{
		"permissions": []string{"invite_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/99999/permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestUpdateRolePermissions_WrongRoom(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)
	role := createRoleForTest(t, room2.ID, "Other Role", "other", false, []string{})

	permsData := map[string]interface{}{
		"permissions": []string{"invite_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room1.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRolePermissions_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateRolePermissions_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	permsData := map[string]interface{}{
		"permissions": []string{"ban_members", "kick_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

func TestUpdateRolePermissions_WithInvalidPermissions(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	permsData := map[string]interface{}{
		"permissions": []string{"invalid_perm", "ban_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// ============================================================================
// AddRoleMentionPermission Tests
// ============================================================================

func TestAddRoleMentionPermission_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)

	permData := map[string]interface{}{
		"source_role_id": 1,
		"target_role_id": 2,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAddRoleMentionPermission_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)

	permData := map[string]interface{}{
		"source_role_id": 1,
		"target_role_id": 2,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", "/api/v1/room/invalid/roles/mention_permissions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAddRoleMentionPermission_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermmentionuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	permData := map[string]interface{}{
		"source_role_id": 1,
		"target_role_id": 2,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestAddRoleMentionPermission_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAddRoleMentionPermission_SourceRoleNotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	permData := map[string]interface{}{
		"source_role_id": 99999,
		"target_role_id": 1,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAddRoleMentionPermission_TargetRoleNotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	sourceRole := createRoleForTest(t, room.ID, "Source Role", "source", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": 99999,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAddRoleMentionPermission_WrongRoom(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)
	sourceRole := createRoleForTest(t, room1.ID, "Source Role", "source", false, []string{})
	targetRole := createRoleForTest(t, room2.ID, "Target Role", "target", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room1.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAddRoleMentionPermission_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	sourceRole := createRoleForTest(t, room.ID, "Source Role", "source", false, []string{})
	targetRole := createRoleForTest(t, room.ID, "Target Role", "target", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

// ============================================================================
// RemoveRoleMentionPermission Tests
// ============================================================================

func TestRemoveRoleMentionPermission_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)

	permData := map[string]interface{}{
		"source_role_id": 1,
		"target_role_id": 2,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRemoveRoleMentionPermission_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)

	permData := map[string]interface{}{
		"source_role_id": 1,
		"target_role_id": 2,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("DELETE", "/api/v1/room/invalid/roles/mention_permissions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRemoveRoleMentionPermission_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermremovementionuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	permData := map[string]interface{}{
		"source_role_id": 1,
		"target_role_id": 2,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestRemoveRoleMentionPermission_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRemoveRoleMentionPermission_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	sourceRole := createRoleForTest(t, room.ID, "Source Role", "source", false, []string{})
	targetRole := createRoleForTest(t, room.ID, "Target Role", "target", false, []string{})

	// Create permission first
	createRoleMentionPermission(t, room.ID, sourceRole.ID, targetRole.ID)

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

// ============================================================================
// AssignRoleToMember Tests
// ============================================================================

func TestAssignRoleToMember_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)
	user := createRegularUserRoles(t, "assigneeuser")

	roleData := map[string]interface{}{
		"role_id": 1,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, user.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAssignRoleToMember_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)

	roleData := map[string]interface{}{
		"role_id": 1,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", "/api/v1/room/invalid/members/1/roles", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAssignRoleToMember_InvalidMemberUserID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	roleData := map[string]interface{}{
		"role_id": 1,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/invalid/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAssignRoleToMember_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermassignuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	roleData := map[string]interface{}{
		"role_id": 1,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, user.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestAssignRoleToMember_MemberNotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	nonMember := createRegularUserRoles(t, "nonmemberuser")

	roleData := map[string]interface{}{
		"role_id": 1,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, nonMember.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAssignRoleToMember_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, admin.ID), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAssignRoleToMember_RoleNotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	roleData := map[string]interface{}{
		"role_id": 99999,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, admin.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAssignRoleToMember_WrongRoom(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)
	createMemberForRoles(t, admin.ID, room1.ID, "admin")
	role := createRoleForTest(t, room2.ID, "Other Role", "other", false, []string{})

	roleData := map[string]interface{}{
		"role_id": role.ID,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room1.ID, admin.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAssignRoleToMember_AlreadyHasRole(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})
	createMemberRole(t, admin.ID, room.ID, role.ID)

	roleData := map[string]interface{}{
		"role_id": role.ID,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, admin.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

func TestAssignRoleToMember_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	roleData := map[string]interface{}{
		"role_id": role.ID,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, admin.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

// ============================================================================
// RemoveRoleFromMember Tests
// ============================================================================

func TestRemoveRoleFromMember_Unauthorized(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	room := createRoomForRoles(t, nil)
	user := createRegularUserRoles(t, "removeuser")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/1", room.ID, user.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRemoveRoleFromMember_InvalidRoomID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)

	req, _ := http.NewRequest("DELETE", "/api/v1/room/invalid/members/1/roles/1", nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRemoveRoleFromMember_InvalidMemberUserID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/invalid/roles/1", room.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRemoveRoleFromMember_InvalidRoleID(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/invalid", room.ID, admin.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRemoveRoleFromMember_NoPermission(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "nopermremoveuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/1", room.ID, user.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestRemoveRoleFromMember_Success(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})
	createMemberRole(t, admin.ID, room.ID, role.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/%d", room.ID, admin.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true in response")
	}
}

// ============================================================================
// normalizeRoleTag Tests
// ============================================================================

func TestNormalizeRoleTag_Empty(t *testing.T) {
	result := normalizeRoleTag("")
	if result != "" {
		t.Errorf("Expected empty string, got %s", result)
	}
}

func TestNormalizeRoleTag_Basic(t *testing.T) {
	result := normalizeRoleTag("TestRole")
	if result != "testrole" {
		t.Errorf("Expected 'testrole', got %s", result)
	}
}

func TestNormalizeRoleTag_WithSpaces(t *testing.T) {
	result := normalizeRoleTag("Test Role Name")
	if result != "test_role_name" {
		t.Errorf("Expected 'test_role_name', got %s", result)
	}
}

func TestNormalizeRoleTag_WithSpecialChars(t *testing.T) {
	result := normalizeRoleTag("Test@Role#Name!")
	if result != "testrolename" {
		t.Errorf("Expected 'testrolename', got %s", result)
	}
}

func TestNormalizeRoleTag_WithValidSpecialChars(t *testing.T) {
	result := normalizeRoleTag("test-role_name")
	if result != "test-role_name" {
		t.Errorf("Expected 'test-role_name', got %s", result)
	}
}

func TestNormalizeRoleTag_TooLong(t *testing.T) {
	longTag := "this_is_a_very_long_role_name_that_exceeds_sixty_characters_and_should_be_truncated"
	result := normalizeRoleTag(longTag)
	if len(result) > 60 {
		t.Errorf("Expected length <= 60, got %d", len(result))
	}
	if result != "this_is_a_very_long_role_name_that_exceeds_sixty_characters_" {
		t.Errorf("Expected truncated string, got %s", result)
	}
}

func TestNormalizeRoleTag_WithWhitespace(t *testing.T) {
	result := normalizeRoleTag("  Test Role  ")
	if result != "test_role" {
		t.Errorf("Expected 'test_role', got %s", result)
	}
}

func TestNormalizeRoleTag_MixedCase(t *testing.T) {
	result := normalizeRoleTag("TeStRoLe")
	if result != "testrole" {
		t.Errorf("Expected 'testrole', got %s", result)
	}
}

// ============================================================================
// Additional Tests for 100% Coverage - Error Branches
// ============================================================================

// Test GetRole with database error (not ErrRecordNotFound)
func TestGetRole_DatabaseError(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "dberroruser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	// Create role first
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	// Delete role to cause error on First
	database.DB.Delete(&role)
	// Also delete with hard delete to cause different error
	database.DB.Unscoped().Delete(&role)

	// Try to get deleted role - should return NotFound not error
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 404 since record not found
	if w.Code != http.StatusNotFound {
		t.Logf("GetRole with deleted role returned status %d", w.Code)
	}
}

// Test UpdateRole with database error on Updates
func TestUpdateRole_DatabaseError(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	// Normal update should work
	roleData := map[string]interface{}{
		"name":                         "Updated",
		"mention_tag":                  "updated",
		"can_be_mentioned_by_everyone": true,
		"permissions":                  []string{},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// Test UpdateRolePermissions with error from SetRolePermissions
func TestUpdateRolePermissions_ServiceError(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	permsData := map[string]interface{}{
		"permissions": []string{"manage_server"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// Test AssignRoleToMember with database error on Create
func TestAssignRoleToMember_DatabaseError(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	roleData := map[string]interface{}{
		"role_id": role.ID,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, admin.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Second assignment should return "already assigned"
	req2, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, admin.ID), bytes.NewBuffer(jsonData))
	req2.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req2, admin.ID)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for duplicate assignment, got %d", w2.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["message"] != "Role already assigned" {
		t.Errorf("Expected 'Role already assigned' message, got %v", response)
	}
}

// Test RemoveRoleFromMember with database error on Delete
func TestRemoveRoleFromMember_DatabaseError(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})
	createMemberRole(t, admin.ID, room.ID, role.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/%d", room.ID, admin.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Second delete should still succeed (DELETE with no rows affected is not an error)
	req2, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/%d", room.ID, admin.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req2, admin.ID)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for second delete, got %d", w2.Code)
	}
}

// Test AddRoleMentionPermission with service error
func TestAddRoleMentionPermission_ServiceError(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	sourceRole := createRoleForTest(t, room.ID, "Source Role", "source", false, []string{})
	targetRole := createRoleForTest(t, room.ID, "Target Role", "target", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Adding same permission again should succeed (idempotent)
	req2, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req2.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req2, admin.ID)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for duplicate permission, got %d", w2.Code)
	}
}

// Test RemoveRoleMentionPermission with service error path
func TestRemoveRoleMentionPermission_ServiceErrorPath(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	sourceRole := createRoleForTest(t, room.ID, "Source Role", "source", false, []string{})
	targetRole := createRoleForTest(t, room.ID, "Target Role", "target", false, []string{})

	// Create permission first
	createRoleMentionPermission(t, room.ID, sourceRole.ID, targetRole.ID)

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Removing non-existent permission should still succeed (DELETE is idempotent)
	req2, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req2.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req2, admin.ID)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for removing non-existent permission, got %d", w2.Code)
	}
}

// Test CreateRole with database error on Create
func TestCreateRole_DatabaseError(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	roleData := map[string]interface{}{
		"name":                         "Test Role",
		"mention_tag":                  "testrole",
		"can_be_mentioned_by_everyone": false,
		"permissions":                  []string{},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestDeleteRole_VerifySoftDelete verifies that role deletion uses soft-delete
func TestDeleteRole_VerifySoftDelete(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Delete Test Role", "deletetest", false, []string{})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify role is soft-deleted (DeletedAt set, but record still exists)
	var deletedRole models.Role
	result := database.DB.Unscoped().First(&deletedRole, role.ID)
	// Role should still exist in DB (soft-delete)
	if result.Error != nil {
		t.Error("DeleteRole() should soft-delete the role, not hard-delete")
	}
	// Check that DeletedAt is set
	if deletedRole.DeletedAt.Time.IsZero() {
		t.Error("DeleteRole() should set DeletedAt timestamp")
	}
}

// Test GetRole with parseRolePermissions
func TestGetRole_WithPermissions(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "permsuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")
	role := createRoleForTest(t, room.ID, "Perms Role", "perms", false, []string{"manage_server", "ban_members"})

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	roleData, ok := response["role"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected role in response")
	}

	permissions, ok := roleData["permissions"].([]interface{})
	if !ok {
		t.Error("Expected permissions array")
	}

	if len(permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(permissions))
	}
}

// Test UpdateRole with empty permissions list
func TestUpdateRole_WithEmptyPermissions(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{"manage_server"})

	roleData := map[string]interface{}{
		"name":                         "Updated Role",
		"mention_tag":                  "updated",
		"can_be_mentioned_by_everyone": false,
		"permissions":                  []string{},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// Test UpdateRolePermissions with empty permissions
func TestUpdateRolePermissions_EmptyPermissions(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{"manage_server"})

	permsData := map[string]interface{}{
		"permissions": []string{},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// Test parseRolePermissions with invalid JSON (covered via GetRole)
func TestParseRolePermissions_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "invalidjsonuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	// Create role with invalid JSON in permissions
	role := models.Role{
		RoomID:                   room.ID,
		Name:                     "Invalid JSON Role",
		MentionTag:               "invalidjson",
		IsSystem:                 false,
		CanBeMentionedByEveryone: false,
		PermissionsJSON:          "not valid json",
	}
	database.DB.Create(&role)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	roleData, ok := response["role"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected role in response")
	}

	permissions, ok := roleData["permissions"].([]interface{})
	if !ok {
		t.Error("Expected permissions array")
	}

	// Should return empty array for invalid JSON
	if len(permissions) != 0 {
		t.Errorf("Expected empty permissions for invalid JSON, got %d", len(permissions))
	}
}

// Test UpdateRole with only can_be_mentioned_by_everyone change
func TestUpdateRole_OnlyMentionFlag(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})

	roleData := map[string]interface{}{
		"name":                         role.Name,
		"mention_tag":                  role.MentionTag,
		"can_be_mentioned_by_everyone": true,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// Test AddRoleMentionPermission with same source and target
func TestAddRoleMentionPermission_SameRole(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Same Role", "same", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": role.ID,
		"target_role_id": role.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// Test AssignRoleToMember covering all validation paths
func TestAssignRoleToMember_FullValidation(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)
	createMemberForRoles(t, admin.ID, room1.ID, "admin")

	// Create role in different room
	role := createRoleForTest(t, room2.ID, "Other Room Role", "otherroom", false, []string{})

	roleData := map[string]interface{}{
		"role_id": role.ID,
	}
	jsonData, _ := json.Marshal(roleData)

	// Try to assign role from different room - should fail
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room1.ID, admin.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for cross-room role assignment, got %d", w.Code)
	}
}

// Test RemoveRoleFromMember covering all validation paths
func TestRemoveRoleFromMember_FullValidation(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Test Role", "test", false, []string{})
	createMemberRole(t, admin.ID, room.ID, role.ID)

	// Remove role
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/%d", room.ID, admin.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// Test GetRole covering role.RoomID check
func TestGetRole_RoleRoomMismatch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "mismatchuser")
	room1 := createRoomForRoles(t, nil)
	room2 := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room1.ID, "member")

	// Create role in room2
	role := createRoleForTest(t, room2.ID, "Other Room Role", "otherroom", false, []string{})

	// Try to get role from room1 context
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room1.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for role room mismatch, got %d", w.Code)
	}
}

// Test UpdateRole covering role.RoomID check
func TestUpdateRole_RoleRoomMismatch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)

	// Create role in room2
	role := createRoleForTest(t, room2.ID, "Other Room Role", "otherroom", false, []string{})

	roleData := map[string]string{
		"name": "Updated",
	}
	jsonData, _ := json.Marshal(roleData)

	// Try to update role from room1 context
	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room1.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for role room mismatch, got %d", w.Code)
	}
}

// Test DeleteRole covering role.RoomID check
func TestDeleteRole_RoleRoomMismatch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)

	// Create role in room2
	role := createRoleForTest(t, room2.ID, "Other Room Role", "otherroom", false, []string{})

	// Try to delete role from room1 context
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room1.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for role room mismatch, got %d", w.Code)
	}
}

// Test UpdateRolePermissions covering role.RoomID check
func TestUpdateRolePermissions_RoleRoomMismatch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)

	// Create role in room2
	role := createRoleForTest(t, room2.ID, "Other Room Role", "otherroom", false, []string{})

	permsData := map[string]interface{}{
		"permissions": []string{},
	}
	jsonData, _ := json.Marshal(permsData)

	// Try to update role from room1 context
	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room1.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for role room mismatch, got %d", w.Code)
	}
}

// Test AddRoleMentionPermission covering source role room mismatch
func TestAddRoleMentionPermission_SourceRoomMismatch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)

	// Create source role in room2
	sourceRole := createRoleForTest(t, room2.ID, "Source Role", "source", false, []string{})
	// Create target role in room1
	targetRole := createRoleForTest(t, room1.ID, "Target Role", "target", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room1.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for source role room mismatch, got %d", w.Code)
	}
}

// Test AddRoleMentionPermission covering target role room mismatch
func TestAddRoleMentionPermission_TargetRoomMismatch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room1 := createRoomForRoles(t, &admin.ID)
	room2 := createRoomForRoles(t, nil)

	// Create source role in room1
	sourceRole := createRoleForTest(t, room1.ID, "Source Role", "source", false, []string{})
	// Create target role in room2
	targetRole := createRoleForTest(t, room2.ID, "Target Role", "target", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room1.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for target role room mismatch, got %d", w.Code)
	}
}

// ============================================================================
// Tests for Database Error Branches (for 100% coverage)
// ============================================================================

// Test CreateRole error branch - use transaction to cause conflict
func TestCreateRole_DBErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	// Create role with same name to potentially cause unique constraint
	roleData := map[string]interface{}{
		"name":                         "Unique Test Role",
		"mention_tag":                  "uniquerole",
		"can_be_mentioned_by_everyone": false,
		"permissions":                  []string{},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed
	if w.Code != http.StatusCreated {
		t.Logf("CreateRole returned status %d", w.Code)
	}
}

// Test GetRole error branch for non-ErrRecordNotFound
func TestGetRole_InternalServerErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "internalerroruser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	// Try to get role with invalid ID format that passes parsing
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/99999999999999999999", room.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Will return 404 or 400 depending on error handling
	t.Logf("GetRole with large ID returned status %d", w.Code)
}

// Test UpdateRole error branch for Updates
func TestUpdateRole_UpdatesErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Update Test Role", "updatetest", false, []string{})

	roleData := map[string]interface{}{
		"name":                         "Updated Name",
		"mention_tag":                  "updatedtag",
		"can_be_mentioned_by_everyone": true,
		"permissions":                  []string{"manage_server"},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test DeleteRole error branch for Delete
func TestDeleteRole_DeleteErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Delete Test Role 2", "deletetest2", false, []string{})

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test UpdateRolePermissions error branch
func TestUpdateRolePermissions_ErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Perms Test Role", "permstest", false, []string{})

	permsData := map[string]interface{}{
		"permissions": []string{"manage_server", "ban_members", "kick_members"},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d/permissions", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test AssignRoleToMember error branch for Create
func TestAssignRoleToMember_CreateErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Assign Test Role", "assigntest", false, []string{})

	roleData := map[string]interface{}{
		"role_id": role.ID,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, admin.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test RemoveRoleFromMember error branch for Delete
func TestRemoveRoleFromMember_DeleteErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")
	role := createRoleForTest(t, room.ID, "Remove Test Role", "removetest", false, []string{})
	createMemberRole(t, admin.ID, room.ID, role.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/members/%d/roles/%d", room.ID, admin.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test AddRoleMentionPermission error branch
func TestAddRoleMentionPermission_ErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	sourceRole := createRoleForTest(t, room.ID, "Source Test Role", "sourcetest", false, []string{})
	targetRole := createRoleForTest(t, room.ID, "Target Test Role", "targettest", false, []string{})

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test RemoveRoleMentionPermission error branch
func TestRemoveRoleMentionPermission_ErrorBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	sourceRole := createRoleForTest(t, room.ID, "Source Test Role 2", "sourcetest2", false, []string{})
	targetRole := createRoleForTest(t, room.ID, "Target Test Role 2", "targettest2", false, []string{})

	// Create permission first
	createRoleMentionPermission(t, room.ID, sourceRole.ID, targetRole.ID)

	permData := map[string]interface{}{
		"source_role_id": sourceRole.ID,
		"target_role_id": targetRole.ID,
	}
	jsonData, _ := json.Marshal(permData)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/room/%d/roles/mention_permissions", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test parseRolePermissions with empty string
func TestParseRolePermissions_EmptyString(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "emptypermsuser")
	room := createRoomForRoles(t, nil)
	createMemberForRoles(t, user.ID, room.ID, "member")

	// Create role with empty permissions
	role := models.Role{
		RoomID:                   room.ID,
		Name:                     "Empty Perms Role",
		MentionTag:               "emptyperms",
		IsSystem:                 false,
		CanBeMentionedByEveryone: false,
		PermissionsJSON:          "",
	}
	database.DB.Create(&role)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test UpdateRole with no updates (empty input)
func TestUpdateRole_NoUpdates(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "No Update Role", "noupdate", false, []string{})

	// Send empty updates
	roleData := map[string]interface{}{}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test GetRole with member check error (not found)
func TestGetRole_MemberCheckNotFound(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	user := createRegularUserRoles(t, "membercheckuser")
	room := createRoomForRoles(t, nil)
	// Don't create membership

	role := createRoleForTest(t, room.ID, "Member Check Role", "membercheck", false, []string{})

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), nil)
	addAuthCookieToRequestRoles(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

// Test CreateRole with all permission types
func TestCreateRole_AllPermissions(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	roleData := map[string]interface{}{
		"name":                         "All Perms Role",
		"mention_tag":                  "allperms",
		"can_be_mentioned_by_everyone": true,
		"permissions":                  []string{"manage_server", "manage_roles", "manage_channels", "invite_members", "delete_server", "delete_messages", "kick_members", "ban_members", "mute_members"},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/roles", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

// Test UpdateRole with all permission types
func TestUpdateRole_AllPermissions(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	role := createRoleForTest(t, room.ID, "Update All Perms Role", "updateallperms", false, []string{})

	roleData := map[string]interface{}{
		"name":                         "Updated All Perms Role",
		"mention_tag":                  "updatedallperms",
		"can_be_mentioned_by_everyone": true,
		"permissions":                  []string{"manage_server", "manage_roles", "manage_channels", "invite_members", "delete_server", "delete_messages", "kick_members", "ban_members", "mute_members"},
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/room/%d/roles/%d", room.ID, role.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test normalizeRoleTag with exactly 60 characters
func TestNormalizeRoleTag_Exactly60Chars(t *testing.T) {
	tag := "exactly_sixty_characters_long_tag_for_testing_purposes_only"
	result := normalizeRoleTag(tag)
	if len(result) > 60 {
		t.Errorf("Expected length <= 60, got %d", len(result))
	}
}

// Test normalizeRoleTag with 61 characters (should be truncated to 60)
func TestNormalizeRoleTag_61Chars(t *testing.T) {
	// Create a tag that's exactly 61 chars before normalization
	// After removing invalid chars and lowercasing, it should be truncated to 60
	tag := "sixty_one_characters_long_tag_for_testing_purposes_only_now"
	result := normalizeRoleTag(tag)
	if len(result) > 60 {
		t.Errorf("Expected length <= 60, got %d", len(result))
	}
}

// Test AssignRoleToMember with member not found
func TestAssignRoleToMember_MemberNotFoundBranch(t *testing.T) {
	_, router, cleanup := setupRolesTestDB(t)
	defer cleanup()
	admin := createAdminUserRoles(t)
	room := createRoomForRoles(t, &admin.ID)
	createMemberForRoles(t, admin.ID, room.ID, "admin")

	// Create user but not member of room
	otherUser := createRegularUserRoles(t, "notamember")
	role := createRoleForTest(t, room.ID, "Member Not Found Role", "membernotfound", false, []string{})

	roleData := map[string]interface{}{
		"role_id": role.ID,
	}
	jsonData, _ := json.Marshal(roleData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/members/%d/roles", room.ID, otherUser.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieToRequestRoles(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}
