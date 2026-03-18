package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Test Setup Helpers
// ============================================================================

func setupAPITestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	database.ResetForTesting()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	authHandler := NewAuthHandler(cfg)
	apiHandler := NewAPIHandler(cfg)

	authHandler.RegisterRoutes(router)
	apiHandler.RegisterRoutes(router)

	cleanup := func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}

	return cfg, router, cleanup
}

func setupAPIHandlerWithRoutes(t *testing.T) (*config.Config, *gin.Engine, *APIHandler, func()) {
	t.Helper()

	cfg, router, cleanup := setupAPITestDB(t)
	apiHandler := NewAPIHandler(cfg)

	return cfg, router, apiHandler, cleanup
}

func hashPasswordForAPI(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

func createTestUserForAPI(t *testing.T, username string, isSuperuser bool) *models.User {
	t.Helper()
	hashedPassword := hashPasswordForAPI(t, "password123")
	user := models.User{
		Username:       username,
		Password:       hashedPassword,
		PresenceStatus: "offline",
		IsSuperuser:    isSuperuser,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	return &user
}

func createTestRoomForAPI(t *testing.T, name, roomType string, ownerID uint) *models.Room {
	t.Helper()
	room := models.Room{
		Name:    name,
		Type:    roomType,
		OwnerID: &ownerID,
		InviteToken: fmt.Sprintf("token_%s_%d", strings.ReplaceAll(strings.ToLower(name), " ", "_"), time.Now().UnixNano()),
	}
	if err := database.DB.Create(&room).Error; err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	return &room
}

func createTestChannelForAPI(t *testing.T, name string, roomID uint) *models.Channel {
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

func createTestMemberForAPI(t *testing.T, userID, roomID uint, role string) *models.Member {
	t.Helper()
	member := models.Member{
		UserID: userID,
		RoomID: roomID,
		Role:   role,
	}
	if err := database.DB.Create(&member).Error; err != nil {
		t.Fatalf("Failed to create member: %v", err)
	}
	return &member
}

func createTestMessageForAPI(t *testing.T, content string, userID, channelID uint, messageType string) *models.Message {
	t.Helper()
	if messageType == "" {
		messageType = "text"
	}
	message := models.Message{
		Content:     content,
		UserID:      userID,
		ChannelID:   channelID,
		MessageType: messageType,
	}
	if err := database.DB.Create(&message).Error; err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}
	return &message
}

func addAuthCookieForAPI(req *http.Request, userID uint, username string) {
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", userID),
	})
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uname",
		Value: username,
	})
}

// ============================================================================
// NewAPIHandler Tests
// ============================================================================

func TestNewAPIHandler_Success(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	handler := NewAPIHandler(cfg)

	if handler == nil {
		t.Fatal("NewAPIHandler() should return non-nil handler")
	}
	if handler.cfg == nil {
		t.Error("NewAPIHandler() should initialize cfg")
	}
	if handler.giphyService == nil {
		t.Error("NewAPIHandler() should initialize giphyService")
	}
}

func TestAPIHandler_RegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	handler := NewAPIHandler(cfg)
	router := gin.New()
	handler.RegisterRoutes(router)

	routes := router.Routes()

	expectedRoutes := map[string]bool{
		"/api/v1/user/me":                        false,
		"/api/v1/user/settings":                  false,
		"/api/v1/user/avatar":                    false,
		"/api/v1/rooms":                          false,
		"/api/v1/room/:room_id":                  false,
		"/api/v1/room/:room_id/join":             false,
		"/api/v1/room/:room_id/members":          false,
		"/api/v1/channel/:channel_id/messages":   false,
		"/api/v1/channel/:channel_id/mark_read":  false,
		"/api/v1/message/:message_id/reaction":   false,
		"/api/v1/message/:message_id/delete":     false,
		"/api/v1/message/:message_id/edit":       false,
		"/api/v1/message/:message_id/forward":    false,
		"/api/v1/reactions":                      false,
		"/api/v1/gifs/trending":                  false,
		"/api/v1/gifs/search":                    false,
		"/upload_file":                           false,
		"/channels/accessible":                   false,
	}

	for _, route := range routes {
		if _, exists := expectedRoutes[route.Path]; exists {
			expectedRoutes[route.Path] = true
		}
	}

	for path, found := range expectedRoutes {
		if !found {
			t.Errorf("Route %s not registered", path)
		}
	}
}

// ============================================================================
// GetCurrentUser Tests
// ============================================================================

func TestGetCurrentUser_Authenticated(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "currentuser", false)
	user.Bio = "Test bio"
	user.AvatarURL = "/uploads/avatars/test.jpg"
	user.PrivacySearchable = true
	user.PrivacyListable = false
	user.HideStatus = true
	user.PresenceStatus = "hidden"
	database.DB.Save(user)

	req, _ := http.NewRequest("GET", "/api/v1/user/me", nil)
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

	if response["id"] != float64(user.ID) {
		t.Errorf("Expected id=%d, got %v", user.ID, response["id"])
	}
	if response["username"] != "currentuser" {
		t.Errorf("Expected username='currentuser', got %v", response["username"])
	}
	if response["bio"] != "Test bio" {
		t.Errorf("Expected bio='Test bio', got %v", response["bio"])
	}
	if response["avatar_url"] != "/uploads/avatars/test.jpg" {
		t.Errorf("Expected avatar_url='/uploads/avatars/test.jpg', got %v", response["avatar_url"])
	}
}

func TestGetCurrentUser_Unauthenticated(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/user/me", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetCurrentUser_WithSuperuser(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "superuser", true)

	req, _ := http.NewRequest("GET", "/api/v1/user/me", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// API returns user data directly, not nested under "user"
	username, ok := response["username"].(string)
	if !ok {
		t.Fatal("Expected username in response")
	}
	if username != "superuser" {
		t.Errorf("Expected username 'superuser', got '%s'", username)
	}
}

// ============================================================================
// UpdateUserSettings Tests
// ============================================================================

func TestUpdateUserSettings_Bio(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "settingsuser", false)

	updateData := map[string]string{
		"bio": "Updated bio",
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify bio was updated
	var updatedUser models.User
	database.DB.First(&updatedUser, user.ID)
	if updatedUser.Bio != "Updated bio" {
		t.Errorf("Expected bio='Updated bio', got '%s'", updatedUser.Bio)
	}
}

func TestUpdateUserSettings_BioTooLong(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "longbiouser", false)

	longBio := ""
	for i := 0; i < 350; i++ {
		longBio += "a"
	}

	updateData := map[string]string{
		"bio": longBio,
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify bio was truncated to 300
	var updatedUser models.User
	database.DB.First(&updatedUser, user.ID)
	if len(updatedUser.Bio) != 300 {
		t.Errorf("Expected bio length 300, got %d", len(updatedUser.Bio))
	}
}

func TestUpdateUserSettings_Username(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "oldusername", false)

	updateData := map[string]string{
		"username": "newusername",
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify username was updated
	var updatedUser models.User
	database.DB.First(&updatedUser, user.ID)
	if updatedUser.Username != "newusername" {
		t.Errorf("Expected username='newusername', got '%s'", updatedUser.Username)
	}
}

func TestUpdateUserSettings_UsernameTooShort(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "testuser", false)

	updateData := map[string]string{
		"username": "ab",
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateUserSettings_UsernameTooLong(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "testuser", false)

	updateData := map[string]string{
		"username": "verylongusernamethatexceedsthirtycharacters",
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestUpdateUserSettings_UsernameTaken(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user1 := createTestUserForAPI(t, "user1", false)
	createTestUserForAPI(t, "user2", false)

	updateData := map[string]string{
		"username": "user2",
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user1.ID, user1.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestUpdateUserSettings_PrivacySettings(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "privacyuser", false)

	updateData := map[string]interface{}{
		"privacy_searchable": false,
		"privacy_listable":   false,
		"hide_status":        true,
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify settings were updated
	var updatedUser models.User
	database.DB.First(&updatedUser, user.ID)
	if updatedUser.PrivacySearchable != false {
		t.Error("Expected privacy_searchable=false")
	}
	if updatedUser.PrivacyListable != false {
		t.Error("Expected privacy_listable=false")
	}
	if updatedUser.HideStatus != true {
		t.Error("Expected hide_status=true")
	}
	if updatedUser.PresenceStatus != "hidden" {
		t.Errorf("Expected presence_status='hidden', got '%s'", updatedUser.PresenceStatus)
	}
}

func TestUpdateUserSettings_Unauthenticated(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	updateData := map[string]string{
		"bio": "Test bio",
	}
	jsonData, _ := json.Marshal(updateData)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestUpdateUserSettings_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "invalidjsonuser", false)

	req, _ := http.NewRequest("PATCH", "/api/v1/user/settings", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// ListRooms Tests
// ============================================================================

func TestListRooms_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "roomuser", false)
	room1 := createTestRoomForAPI(t, "Room 1", "server", user.ID)
	room2 := createTestRoomForAPI(t, "Room 2", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room1.ID, "member")
	createTestMemberForAPI(t, user.ID, room2.ID, "member")

	req, _ := http.NewRequest("GET", "/api/v1/rooms", nil)
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

	rooms, ok := response["rooms"].([]interface{})
	if !ok {
		t.Fatal("Expected rooms array in response")
	}

	if len(rooms) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(rooms))
	}
}

func TestListRooms_NoRooms(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "noroomsuser", false)

	req, _ := http.NewRequest("GET", "/api/v1/rooms", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	rooms, ok := response["rooms"].([]interface{})
	if !ok {
		t.Fatal("Expected rooms array in response")
	}

	if len(rooms) != 0 {
		t.Errorf("Expected 0 rooms, got %d", len(rooms))
	}
}

func TestListRooms_Unauthenticated(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/rooms", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

// ============================================================================
// GetRoom Tests
// ============================================================================

func TestGetRoom_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "getroomuser", false)
	room := createTestRoomForAPI(t, "Test Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d", room.ID), nil)
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

	roomData, ok := response["room"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected room object in response")
	}

	if roomData["name"] != "Test Room" {
		t.Errorf("Expected name='Test Room', got %v", roomData["name"])
	}
	_ = channel // channel should be preloaded
}

func TestGetRoom_NotFound(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notfounduser", false)

	req, _ := http.NewRequest("GET", "/api/v1/room/999", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetRoom_InvalidID(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "invalididuser", false)

	req, _ := http.NewRequest("GET", "/api/v1/room/invalid", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// JoinRoom Tests
// ============================================================================

func TestJoinRoom_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "joinroomuser", false)
	room := createTestRoomForAPI(t, "Joinable Room", "server", user.ID)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/join", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify membership was created
	var member models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", user.ID, room.ID).First(&member)
	if result.Error != nil {
		t.Error("Expected membership to be created")
	}
	if member.Role != "member" {
		t.Errorf("Expected role='member', got '%s'", member.Role)
	}
}

func TestJoinRoom_AlreadyMember(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "alreadymemberuser", false)
	room := createTestRoomForAPI(t, "Already Joined Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/join", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if response["already_member"] != true {
		t.Error("Expected already_member=true")
	}
}

func TestJoinRoom_InvalidID(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "invalidjoinuser", false)

	req, _ := http.NewRequest("POST", "/api/v1/room/invalid/join", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// GetRoomMembers Tests
// ============================================================================

func TestGetRoomMembers_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user1 := createTestUserForAPI(t, "member1user", false)
	user2 := createTestUserForAPI(t, "member2user", false)
	room := createTestRoomForAPI(t, "Members Room", "server", user1.ID)
	createTestMemberForAPI(t, user1.ID, room.ID, "owner")
	createTestMemberForAPI(t, user2.ID, room.ID, "member")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/members", room.ID), nil)
	addAuthCookieForAPI(req, user1.ID, user1.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	members, ok := response["members"].([]interface{})
	if !ok {
		t.Fatal("Expected members array in response")
	}

	if len(members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(members))
	}
}

func TestGetRoomMembers_Empty(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "emptyroomuser", false)
	room := createTestRoomForAPI(t, "Empty Room", "server", user.ID)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/room/%d/members", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// GetChannelMessages Tests
// ============================================================================

func TestGetChannelMessages_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "messagesuser", false)
	room := createTestRoomForAPI(t, "Messages Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg1 := createTestMessageForAPI(t, "Message 1", user.ID, channel.ID, "text")
	msg2 := createTestMessageForAPI(t, "Message 2", user.ID, channel.ID, "text")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/channel/%d/messages?limit=10&offset=0", channel.ID), nil)
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

	messages, ok := response["messages"].([]interface{})
	if !ok {
		t.Fatal("Expected messages array in response")
	}

	if len(messages) == 0 {
		t.Error("Expected at least 1 message")
	}
	_ = msg1
	_ = msg2
}

func TestGetChannelMessages_Limit(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "limituser", false)
	room := createTestRoomForAPI(t, "Limit Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	for i := 0; i < 10; i++ {
		createTestMessageForAPI(t, fmt.Sprintf("Message %d", i), user.ID, channel.ID, "text")
	}

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/channel/%d/messages?limit=5", channel.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	messages, ok := response["messages"].([]interface{})
	if !ok {
		t.Fatal("Expected messages array")
	}

	if len(messages) > 5 {
		t.Errorf("Expected max 5 messages, got %d", len(messages))
	}
}

func TestGetChannelMessages_InvalidID(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "invalidchanneluser", false)

	req, _ := http.NewRequest("GET", "/api/v1/channel/invalid/messages", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// MarkChannelRead Tests
// ============================================================================

func TestMarkChannelRead_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "readuser", false)
	room := createTestRoomForAPI(t, "Read Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "Read this", user.ID, channel.ID, "text")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/channel/%d/mark_read", channel.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify read status was updated
	var readMsg models.ReadMessage
	result := database.DB.Where("user_id = ? AND channel_id = ?", user.ID, channel.ID).First(&readMsg)
	if result.Error != nil {
		t.Error("Expected read status to be created")
	}
	if readMsg.LastReadMessageID == nil || *readMsg.LastReadMessageID != msg.ID {
		t.Error("Expected last read message ID to be set")
	}
}

func TestMarkChannelRead_EmptyChannel(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "emptychanneluser", false)
	room := createTestRoomForAPI(t, "Empty Channel Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "empty", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/channel/%d/mark_read", channel.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// AddReaction Tests
// ============================================================================

func TestAddReaction_Add(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "reactionuser", false)
	room := createTestRoomForAPI(t, "Reaction Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "React to this", user.ID, channel.ID, "text")

	reactionData := map[string]string{
		"emoji":        "👍",
		"reaction_type": "positive",
	}
	jsonData, _ := json.Marshal(reactionData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/reaction", msg.ID), bytes.NewBuffer(jsonData))
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

	if response["action"] != "added" {
		t.Errorf("Expected action='added', got %v", response["action"])
	}

	// Verify reaction was created
	var reaction models.MessageReaction
	result := database.DB.Where("message_id = ? AND user_id = ?", msg.ID, user.ID).First(&reaction)
	if result.Error != nil {
		t.Error("Expected reaction to be created")
	}
	if reaction.Emoji != "👍" {
		t.Errorf("Expected emoji='👍', got '%s'", reaction.Emoji)
	}
}

func TestAddReaction_Remove(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "removereactionuser", false)
	room := createTestRoomForAPI(t, "Remove Reaction Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "React and remove", user.ID, channel.ID, "text")

	// Add reaction first
	reaction := models.MessageReaction{
		MessageID:    msg.ID,
		UserID:       user.ID,
		Emoji:        "❤️",
		ReactionType: "positive",
	}
	database.DB.Create(&reaction)

	reactionData := map[string]string{
		"emoji": "❤️",
	}
	jsonData, _ := json.Marshal(reactionData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/reaction", msg.ID), bytes.NewBuffer(jsonData))
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

	if response["action"] != "removed" {
		t.Errorf("Expected action='removed', got %v", response["action"])
	}

	// Verify reaction was removed
	var deletedReaction models.MessageReaction
	result := database.DB.Where("id = ?", reaction.ID).First(&deletedReaction)
	if result.Error == nil {
		t.Error("Expected reaction to be deleted")
	}
}

func TestAddReaction_MissingEmoji(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "noemojiuser", false)
	room := createTestRoomForAPI(t, "No Emoji Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "No emoji", user.ID, channel.ID, "text")

	reactionData := map[string]string{}
	jsonData, _ := json.Marshal(reactionData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/reaction", msg.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// DeleteMessage Tests
// ============================================================================

func TestDeleteMessage_Owner(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "deletemessageowner", false)
	room := createTestRoomForAPI(t, "Delete Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "Delete me", user.ID, channel.ID, "text")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/delete", msg.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify message was deleted
	var deletedMsg models.Message
	result := database.DB.First(&deletedMsg, msg.ID)
	if result.Error == nil {
		t.Error("Expected message to be deleted")
	}
}

func TestDeleteMessage_Superuser(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "superdeleteuser", true)
	otherUser := createTestUserForAPI(t, "otherdeleteuser", false)
	room := createTestRoomForAPI(t, "Super Delete Room", "server", otherUser.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, otherUser.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "Super delete me", otherUser.ID, channel.ID, "text")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/delete", msg.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteMessage_NotOwner(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user1 := createTestUserForAPI(t, "notowneruser1", false)
	user2 := createTestUserForAPI(t, "notowneruser2", false)
	room := createTestRoomForAPI(t, "Not Owner Room", "server", user1.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user1.ID, room.ID, "member")
	createTestMemberForAPI(t, user2.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "Not your message", user1.ID, channel.ID, "text")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/delete", msg.ID), nil)
	addAuthCookieForAPI(req, user2.ID, user2.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestDeleteMessage_NotFound(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notfoundmsguser", false)

	req, _ := http.NewRequest("POST", "/api/v1/message/999/delete", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// EditMessage Tests
// ============================================================================

func TestEditMessage_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "editmessageuser", false)
	room := createTestRoomForAPI(t, "Edit Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "Original message", user.ID, channel.ID, "text")

	editData := map[string]string{
		"content": "Edited message",
	}
	jsonData, _ := json.Marshal(editData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/edit", msg.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify message was edited
	var editedMsg models.Message
	database.DB.First(&editedMsg, msg.ID)
	if editedMsg.Content != "Edited message" {
		t.Errorf("Expected content='Edited message', got '%s'", editedMsg.Content)
	}
	if editedMsg.EditedAt == nil {
		t.Error("Expected EditedAt to be set")
	}
}

func TestEditMessage_NotOwner(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user1 := createTestUserForAPI(t, "noteditowner1", false)
	user2 := createTestUserForAPI(t, "noteditowner2", false)
	room := createTestRoomForAPI(t, "Not Edit Room", "server", user1.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user1.ID, room.ID, "member")
	createTestMemberForAPI(t, user2.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "Not your message to edit", user1.ID, channel.ID, "text")

	editData := map[string]string{
		"content": "Trying to edit",
	}
	jsonData, _ := json.Marshal(editData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/edit", msg.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user2.ID, user2.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestEditMessage_NotFound(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "editnotfounduser", false)

	editData := map[string]string{
		"content": "Edit non-existent",
	}
	jsonData, _ := json.Marshal(editData)

	req, _ := http.NewRequest("POST", "/api/v1/message/999/edit", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// ForwardMessage Tests
// ============================================================================

func TestForwardMessage_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "forwarduser", false)
	room := createTestRoomForAPI(t, "Forward Room", "server", user.ID)
	channel1 := createTestChannelForAPI(t, "channel1", room.ID)
	channel2 := createTestChannelForAPI(t, "channel2", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	originalMsg := createTestMessageForAPI(t, "Forward this", user.ID, channel1.ID, "text")

	forwardData := map[string]uint{
		"channel_id": channel2.ID,
	}
	jsonData, _ := json.Marshal(forwardData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/forward", originalMsg.ID), bytes.NewBuffer(jsonData))
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

	// Verify forwarded message was created
	var forwardedMsg models.Message
	result := database.DB.Where("channel_id = ? AND user_id = ?", channel2.ID, user.ID).First(&forwardedMsg)
	if result.Error != nil {
		t.Error("Expected forwarded message to be created")
	}
	if forwardedMsg.Content != "Forwarded from "+user.Username+":\nForward this" {
		t.Errorf("Unexpected forwarded content: %s", forwardedMsg.Content)
	}
}

func TestForwardMessage_ChannelNotFound(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "forwardchannelnotfound", false)
	room := createTestRoomForAPI(t, "Forward Channel Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "channel", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	msg := createTestMessageForAPI(t, "Message", user.ID, channel.ID, "text")

	forwardData := map[string]uint{
		"channel_id": 999,
	}
	jsonData, _ := json.Marshal(forwardData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/forward", msg.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestForwardMessage_NoAccess(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user1 := createTestUserForAPI(t, "noaccessuser1", false)
	user2 := createTestUserForAPI(t, "noaccessuser2", false)
	room1 := createTestRoomForAPI(t, "Room 1", "server", user1.ID)
	room2 := createTestRoomForAPI(t, "Room 2", "server", user2.ID)
	channel1 := createTestChannelForAPI(t, "channel1", room1.ID)
	channel2 := createTestChannelForAPI(t, "channel2", room2.ID)
	createTestMemberForAPI(t, user1.ID, room1.ID, "member")
	createTestMemberForAPI(t, user2.ID, room2.ID, "member")

	msg := createTestMessageForAPI(t, "Message", user1.ID, channel1.ID, "text")

	forwardData := map[string]uint{
		"channel_id": channel2.ID,
	}
	jsonData, _ := json.Marshal(forwardData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/message/%d/forward", msg.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user1.ID, user1.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// ListReactions Tests
// ============================================================================

func TestListReactions_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/reactions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	reactions, ok := response["reactions"].([]interface{})
	if !ok {
		t.Fatal("Expected reactions array")
	}

	if len(reactions) == 0 {
		t.Error("Expected at least one reaction")
	}
}

// ============================================================================
// GetAccessibleChannels Tests
// ============================================================================

func TestGetAccessibleChannels_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "accessibleuser", false)
	room1 := createTestRoomForAPI(t, "Accessible Room 1", "server", user.ID)
	room2 := createTestRoomForAPI(t, "Accessible Room 2", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room1.ID, "member")
	createTestMemberForAPI(t, user.ID, room2.ID, "member")

	channel1 := createTestChannelForAPI(t, "general", room1.ID)
	channel2 := createTestChannelForAPI(t, "random", room1.ID)
	channel3 := createTestChannelForAPI(t, "general", room2.ID)

	req, _ := http.NewRequest("GET", "/channels/accessible", nil)
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

	channels, ok := response["channels"].([]interface{})
	if !ok {
		t.Fatal("Expected channels array")
	}

	if len(channels) != 3 {
		t.Errorf("Expected 3 channels, got %d", len(channels))
	}
	_ = channel1
	_ = channel2
	_ = channel3
}

func TestGetAccessibleChannels_NoChannels(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "nochannelsuser", false)
	room := createTestRoomForAPI(t, "No Channels Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("GET", "/channels/accessible", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	channels, ok := response["channels"].([]interface{})
	if !ok {
		t.Fatal("Expected channels array")
	}

	if len(channels) != 0 {
		t.Errorf("Expected 0 channels, got %d", len(channels))
	}
}

// ============================================================================
// DeleteAccount Tests
// ============================================================================

func TestDeleteAccount_Success(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "deleteaccountuser", false)
	room := createTestRoomForAPI(t, "Delete Account Room", "server", user.ID)
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")
	msg := createTestMessageForAPI(t, "Message to delete", user.ID, channel.ID, "text")

	// Create reaction
	reaction := models.MessageReaction{
		MessageID: msg.ID,
		UserID:    user.ID,
		Emoji:     "👍",
	}
	database.DB.Create(&reaction)

	// Create read message
	readMsg := models.ReadMessage{
		UserID:            user.ID,
		ChannelID:         channel.ID,
		LastReadMessageID: &msg.ID,
	}
	database.DB.Create(&readMsg)

	deleteData := map[string]string{
		"password": "password123",
	}
	jsonData, _ := json.Marshal(deleteData)

	req, _ := http.NewRequest("POST", "/api/v1/user/delete", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify user was deleted
	var deletedUser models.User
	result := database.DB.First(&deletedUser, user.ID)
	if result.Error == nil {
		t.Error("Expected user to be deleted")
	}
}

func TestDeleteAccount_WrongPassword(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "wrongpassworduser", false)

	deleteData := map[string]string{
		"password": "wrongpassword",
	}
	jsonData, _ := json.Marshal(deleteData)

	req, _ := http.NewRequest("POST", "/api/v1/user/delete", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestDeleteAccount_MissingPassword(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "missingpassworduser", false)

	deleteData := map[string]string{}
	jsonData, _ := json.Marshal(deleteData)

	req, _ := http.NewRequest("POST", "/api/v1/user/delete", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDeleteAccount_WithStickerPacks(t *testing.T) {
	_, router, cleanup := setupAPITestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "stickerpackuser", false)

	// Create sticker pack
	pack := models.StickerPack{
		Name:       "Test Pack",
		OwnerID:    user.ID,
		IconEmoji:  "😀",
	}
	database.DB.Create(&pack)

	// Create sticker
	sticker := models.Sticker{
		Name:     "Test Sticker",
		PackID:   pack.ID,
		OwnerID:  user.ID,
		FileURL:  "/uploads/stickers/test.png",
	}
	database.DB.Create(&sticker)

	deleteData := map[string]string{
		"password": "password123",
	}
	jsonData, _ := json.Marshal(deleteData)

	req, _ := http.NewRequest("POST", "/api/v1/user/delete", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify sticker pack was deleted
	var deletedPack models.StickerPack
	result := database.DB.First(&deletedPack, pack.ID)
	if result.Error == nil {
		t.Error("Expected sticker pack to be deleted")
	}
}
