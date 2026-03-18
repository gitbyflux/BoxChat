package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/testutil"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Helper Functions for Channels Tests
// ============================================================================

func setupChannelsTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	cfg, dbCleanup := testutil.SetupTestDB(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	apiHandler := NewAPIHandler(cfg)
	apiHandler.RegisterRoutes(router)
	apiHandler.RegisterChannelsRoutes(router)

	return cfg, router, dbCleanup
}

func hashPasswordChannels(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

func createChannelsUser(t *testing.T, username string, isSuperuser bool) *models.User {
	t.Helper()
	hashedPassword := hashPasswordChannels(t, "password123")
	user := models.User{
		Username:    username,
		Password:    hashedPassword,
		IsSuperuser: isSuperuser,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	return &user
}

func createChannelsRoom(t *testing.T, ownerID *uint) *models.Room {
	t.Helper()
	room := models.Room{
		Name:    "Test Room",
		Type:    "server",
		OwnerID: ownerID,
	}
	if err := database.DB.Create(&room).Error; err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	return &room
}

func createChannelsMember(t *testing.T, userID, roomID uint, role string) {
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

func createChannelsChannel(t *testing.T, roomID uint, name string) *models.Channel {
	t.Helper()
	channel := models.Channel{
		Name:   name,
		RoomID: roomID,
	}
	if err := database.DB.Create(&channel).Error; err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}
	return &channel
}

func addAuthCookieChannels(req *http.Request, userID uint) {
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", userID),
	})
}

// ============================================================================
// AddChannel Tests
// ============================================================================

func TestAddChannel_Unauthorized(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channeluser1", false)
	room := createChannelsRoom(t, &user.ID)

	channelData := map[string]string{
		"name": "Test Channel",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/add_channel", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAddChannel_NoPermission(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channeluser2", false)
	room := createChannelsRoom(t, &user.ID)
	createChannelsMember(t, user.ID, room.ID, "member")

	channelData := map[string]string{
		"name": "Test Channel",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/add_channel", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestAddChannel_RoomNotFound(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channeladmin", true)

	channelData := map[string]string{
		"name": "Test Channel",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("POST", "/api/v1/room/99999/add_channel", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAddChannel_Success(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channeladmin2", true)
	room := createChannelsRoom(t, &admin.ID)
	createChannelsMember(t, admin.ID, room.ID, "admin")

	channelData := map[string]string{
		"name":        "general",
		"description": "General chat",
		"icon_emoji":  "💬",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/add_channel", room.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
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
		t.Error("Expected success=true")
	}

	channel, ok := response["channel"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected channel in response")
	}

	if channel["name"] != "general" {
		t.Errorf("Expected channel name 'general', got %v", channel["name"])
	}
}

func TestAddChannel_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channeladmin3", true)
	room := createChannelsRoom(t, &admin.ID)
	createChannelsMember(t, admin.ID, room.ID, "admin")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/add_channel", room.ID), bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// EditChannel Tests
// ============================================================================

func TestEditChannel_Unauthorized(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channeledit1", false)
	room := createChannelsRoom(t, &user.ID)
	channel := createChannelsChannel(t, room.ID, "general")

	channelData := map[string]string{
		"name": "Updated Channel",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/edit", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestEditChannel_NoPermission(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channeledit2", false)
	room := createChannelsRoom(t, &user.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, user.ID, room.ID, "member")

	channelData := map[string]string{
		"name": "Updated Channel",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/edit", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestEditChannel_NotFound(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channeleditadmin", true)

	channelData := map[string]string{
		"name": "Updated Channel",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("PATCH", "/api/v1/channel/99999/edit", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestEditChannel_Success(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channeleditadmin2", true)
	room := createChannelsRoom(t, &admin.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, admin.ID, room.ID, "admin")

	channelData := map[string]string{
		"name":        "updated-general",
		"description": "Updated description",
		"icon_emoji":  "🔥",
	}
	jsonData, _ := json.Marshal(channelData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/edit", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
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
		t.Error("Expected success=true")
	}
}

// ============================================================================
// DeleteChannel Tests
// ============================================================================

func TestDeleteChannel_Unauthorized(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channeldelete1", false)
	room := createChannelsRoom(t, &user.ID)
	channel := createChannelsChannel(t, room.ID, "general")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/channel/%d/delete", channel.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestDeleteChannel_NoPermission(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channeldelete2", false)
	room := createChannelsRoom(t, &user.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/channel/%d/delete", channel.ID), nil)
	addAuthCookieChannels(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestDeleteChannel_NotFound(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channeldeleteadmin", true)

	req, _ := http.NewRequest("DELETE", "/api/v1/channel/99999/delete", nil)
	addAuthCookieChannels(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDeleteChannel_Success(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channeldeleteadmin2", true)
	room := createChannelsRoom(t, &admin.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, admin.ID, room.ID, "admin")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/channel/%d/delete", channel.ID), nil)
	addAuthCookieChannels(req, admin.ID)
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
		t.Error("Expected success=true")
	}

	// Verify channel deleted
	var deletedChannel models.Channel
	result := database.DB.First(&deletedChannel, channel.ID)
	if result.Error == nil {
		t.Error("DeleteChannel() should delete the channel")
	}
}

// ============================================================================
// UpdateChannelPermissions Tests
// ============================================================================

func TestUpdateChannelPermissions_Unauthorized(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channelperms1", false)
	room := createChannelsRoom(t, &user.ID)
	channel := createChannelsChannel(t, room.ID, "general")

	permsData := map[string][]uint{
		"writer_role_ids": {1},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/permissions", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestUpdateChannelPermissions_NoPermission(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	user := createChannelsUser(t, "channelperms2", false)
	room := createChannelsRoom(t, &user.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, user.ID, room.ID, "member")

	permsData := map[string][]uint{
		"writer_role_ids": {1},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/permissions", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestUpdateChannelPermissions_InvalidRoleIDs(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channelpermsadmin", true)
	room := createChannelsRoom(t, &admin.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, admin.ID, room.ID, "admin")

	permsData := map[string][]uint{
		"writer_role_ids": {99999}, // Non-existent role
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/permissions", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateChannelPermissions_Success(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channelpermsadmin2", true)
	room := createChannelsRoom(t, &admin.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, admin.ID, room.ID, "admin")

	// Create a role in the room
	role := models.Role{
		RoomID:     room.ID,
		Name:       "Writer",
		MentionTag: "writer",
		IsSystem:   false,
	}
	database.DB.Create(&role)

	permsData := map[string][]uint{
		"writer_role_ids": {role.ID},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/permissions", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
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
		t.Error("Expected success=true")
	}
}

func TestUpdateChannelPermissions_EmptyRoleIDs(t *testing.T) {
	_, router, cleanup := setupChannelsTestDB(t)
	defer cleanup()

	admin := createChannelsUser(t, "channelpermsadmin3", true)
	room := createChannelsRoom(t, &admin.ID)
	channel := createChannelsChannel(t, room.ID, "general")
	createChannelsMember(t, admin.ID, room.ID, "admin")

	permsData := map[string][]uint{
		"writer_role_ids": {},
	}
	jsonData, _ := json.Marshal(permsData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/channel/%d/permissions", channel.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieChannels(req, admin.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}
