package http

import (
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
// Helper Functions for Search Tests
// ============================================================================

func setupSearchTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	database.ResetForTesting()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_search")

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
	apiHandler.RegisterSearchRoutes(router)

	cleanup := func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}

	return cfg, router, cleanup
}

func hashPasswordSearch(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

func createSearchUser(t *testing.T, username string, searchable bool) *models.User {
	t.Helper()
	hashedPassword := hashPasswordSearch(t, "password123")
	user := models.User{
		Username:          username,
		Password:          hashedPassword,
		PrivacySearchable: searchable,
	}
	// Explicitly save to ensure PrivacySearchable is set correctly (GORM default:true issue)
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	// Update the privacy_searchable field explicitly to work around GORM default:true
	database.DB.Model(&user).Update("privacy_searchable", searchable)
	// Reload user from DB
	database.DB.First(&user, user.ID)
	return &user
}

func createSearchRoom(t *testing.T, name string, isPublic bool, ownerID *uint) *models.Room {
	t.Helper()
	room := models.Room{
		Name:        name,
		Type:        "server",
		IsPublic:    isPublic,
		OwnerID:     ownerID,
		Description: "Test room " + name,
		InviteToken: fmt.Sprintf("token_%s_%d", strings.ReplaceAll(strings.ToLower(name), " ", "_"), time.Now().UnixNano()),
	}
	if err := database.DB.Create(&room).Error; err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	return &room
}

func createSearchMember(t *testing.T, userID, roomID uint) {
	t.Helper()
	member := models.Member{
		UserID: userID,
		RoomID: roomID,
		Role:   "member",
	}
	if err := database.DB.Create(&member).Error; err != nil {
		t.Fatalf("Failed to create member: %v", err)
	}
}

func createSearchChannel(t *testing.T, roomID uint, name string) *models.Channel {
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

func createSearchMessage(t *testing.T, channelID, userID uint, content string) *models.Message {
	t.Helper()
	message := models.Message{
		ChannelID: channelID,
		UserID:    userID,
		Content:   content,
	}
	if err := database.DB.Create(&message).Error; err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}
	return &message
}

func addAuthCookieSearch(req *http.Request, userID uint) {
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", userID),
	})
}

// ============================================================================
// SearchUsers Tests
// ============================================================================

func TestSearchUsers_Unauthorized(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/search/users?q=test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestSearchUsers_EmptyQuery(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchuser1", true)

	req, _ := http.NewRequest("GET", "/api/v1/search/users?q=", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSearchUsers_NoResults(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchuser2", true)

	req, _ := http.NewRequest("GET", "/api/v1/search/users?q=nonexistent", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array")
	}
	if len(users) != 0 {
		t.Errorf("Expected 0 users, got %d", len(users))
	}
}

func TestSearchUsers_FindsUsers(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchuser3", true)
	createSearchUser(t, "alice_search", true)
	createSearchUser(t, "bob_search", true)

	req, _ := http.NewRequest("GET", "/api/v1/search/users?q=search", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array")
	}
	if len(users) < 2 {
		t.Errorf("Expected at least 2 users, got %d", len(users))
	}
}

func TestSearchUsers_ExcludesSelf(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchuser4", true)

	req, _ := http.NewRequest("GET", "/api/v1/search/users?q=searchuser4", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array")
	}
	if len(users) != 0 {
		t.Errorf("Expected 0 users (self excluded), got %d", len(users))
	}
}

func TestSearchUsers_ExcludesNonSearchable(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	// Create searchable user who will perform the search
	searcher := createSearchUser(t, "zzz_search_test_user", true)
	// Create non-searchable user with similar name
	unsearchableUser := createSearchUser(t, "zzz_unsearchable_user", false)
	// Create another searchable user with similar name for comparison
	createSearchUser(t, "zzz_searchable_user", true)
	
	// Verify users were created with correct privacy settings
	var dbUser models.User
	database.DB.First(&dbUser, unsearchableUser.ID)
	t.Logf("Unsearchable user privacy_searchable: %v (expected: false)", dbUser.PrivacySearchable)
	if dbUser.PrivacySearchable != false {
		t.Fatal("Test setup failed: unsearchable user should have privacy_searchable=false")
	}

	req, _ := http.NewRequest("GET", "/api/v1/search/users?q=zzz", nil)
	addAuthCookieSearch(req, searcher.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array")
	}
	
	// Debug: print all found users
	t.Logf("Found %d users", len(users))
	for _, u := range users {
		userMap := u.(map[string]interface{})
		t.Logf("  - %s (id=%d)", userMap["username"], userMap["id"])
	}
	
	// Should find zzz_search_test_user and zzz_searchable_user but NOT zzz_unsearchable_user
	// The key assertion is that the unsearchable user should NOT be in the results
	for _, u := range users {
		userMap := u.(map[string]interface{})
		username := userMap["username"].(string)
		if username == "zzz_unsearchable_user" {
			t.Error("Should not find unsearchable user (privacy_searchable=false)")
		}
	}
	
	// Additionally verify we found the searchable user
	foundSearchable := false
	for _, u := range users {
		userMap := u.(map[string]interface{})
		if userMap["username"] == "zzz_searchable_user" {
			foundSearchable = true
			break
		}
	}
	if !foundSearchable {
		t.Error("Should find searchable user matching the query")
	}
}

func TestSearchUsers_LimitParameter(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchuser6", true)
	for i := 0; i < 10; i++ {
		createSearchUser(t, fmt.Sprintf("limit_user_%d", i), true)
	}

	req, _ := http.NewRequest("GET", "/api/v1/search/users?q=limit_user&limit=5", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array")
	}
	if len(users) > 5 {
		t.Errorf("Expected max 5 users, got %d", len(users))
	}
}

// ============================================================================
// SearchServers Tests
// ============================================================================

func TestSearchServers_PublicServersOnly(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchserver1", true)
	createSearchRoom(t, "Public Server", true, &user.ID)
	createSearchRoom(t, "Private Server", false, &user.ID)

	req, _ := http.NewRequest("GET", "/api/v1/search/servers?q=server", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	servers, ok := response["servers"].([]interface{})
	if !ok {
		t.Fatal("Expected servers array")
	}

	// Should only find public server
	foundPublic := false
	for _, s := range servers {
		server := s.(map[string]interface{})
		if server["name"] == "Public Server" {
			foundPublic = true
		}
		if server["name"] == "Private Server" {
			t.Error("Should not find private server")
		}
	}
	if !foundPublic {
		t.Error("Should find public server")
	}
}

func TestSearchServers_NoQuery(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchserver2", true)
	createSearchRoom(t, "Server No Query", true, &user.ID)

	req, _ := http.NewRequest("GET", "/api/v1/search/servers", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	servers, ok := response["servers"].([]interface{})
	if !ok {
		t.Fatal("Expected servers array")
	}
	if len(servers) < 1 {
		t.Errorf("Expected at least 1 server, got %d", len(servers))
	}
}

func TestSearchServers_WithMemberCount(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchserver3", true)
	room := createSearchRoom(t, "Server With Members", true, &user.ID)
	createSearchMember(t, user.ID, room.ID)
	createSearchUser(t, "member1", true)
	createSearchUser(t, "member2", true)

	req, _ := http.NewRequest("GET", "/api/v1/search/servers?q=members", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	servers, ok := response["servers"].([]interface{})
	if !ok {
		t.Fatal("Expected servers array")
	}
	if len(servers) < 1 {
		t.Fatal("Expected at least 1 server")
	}

	server := servers[0].(map[string]interface{})
	if server["member_count"] == nil {
		t.Error("Expected member_count in response")
	}
}

// ============================================================================
// GlobalSearch Tests
// ============================================================================

func TestGlobalSearch_Unauthorized(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGlobalSearch_EmptyQuery(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "globalsearch1", true)

	req, _ := http.NewRequest("GET", "/api/v1/search?q=", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGlobalSearch_SearchesUsers(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "globalsearch2", true)
	createSearchUser(t, "global_user", true)

	req, _ := http.NewRequest("GET", "/api/v1/search?q=global", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	users, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Expected users array")
	}
	if len(users) < 1 {
		t.Errorf("Expected at least 1 user, got %d", len(users))
	}
}

func TestGlobalSearch_SearchesRooms(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "globalsearch3", true)
	room := createSearchRoom(t, "Global Room", true, &user.ID)
	createSearchMember(t, user.ID, room.ID)

	req, _ := http.NewRequest("GET", "/api/v1/search?q=global", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	rooms, ok := response["rooms"].([]interface{})
	if !ok {
		t.Fatal("Expected rooms array")
	}
	if len(rooms) < 1 {
		t.Errorf("Expected at least 1 room, got %d", len(rooms))
	}
}

func TestGlobalSearch_SearchesMessages(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "globalsearch4", true)
	room := createSearchRoom(t, "Message Room", true, &user.ID)
	createSearchMember(t, user.ID, room.ID)
	channel := createSearchChannel(t, room.ID, "general")
	createSearchMessage(t, channel.ID, user.ID, "global search test message")

	req, _ := http.NewRequest("GET", "/api/v1/search?q=global", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	messages, ok := response["messages"].([]interface{})
	if !ok {
		t.Fatal("Expected messages array")
	}
	if len(messages) < 1 {
		t.Errorf("Expected at least 1 message, got %d", len(messages))
	}
}

func TestGlobalSearch_UserRoomsOnly(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "globalsearch5", true)
	otherUser := createSearchUser(t, "globalsearch5other", true)

	// Room user is NOT a member of
	otherRoom := createSearchRoom(t, "Other Room", true, &otherUser.ID)
	createSearchMember(t, otherUser.ID, otherRoom.ID)

	req, _ := http.NewRequest("GET", "/api/v1/search?q=other", nil)
	addAuthCookieSearch(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	rooms, ok := response["rooms"].([]interface{})
	if !ok {
		t.Fatal("Expected rooms array")
	}
	// Should not find room user is not a member of
	for _, r := range rooms {
		room := r.(map[string]interface{})
		if room["name"] == "Other Room" {
			t.Error("Should not find rooms user is not a member of")
		}
	}
}

// ============================================================================
// Case Insensitivity Tests
// ============================================================================

func TestSearchUsers_CaseInsensitive(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchcase1", true)
	createSearchUser(t, "CaseUser", true)

	// Search with different cases
	testCases := []string{"caseuser", "CASEUSER", "CaseUser"}
	for _, query := range testCases {
		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/search/users?q=%s", query), nil)
		addAuthCookieSearch(req, user.ID)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("SearchUsers(%q) expected status 200, got %d", query, w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		users, ok := response["users"].([]interface{})
		if !ok {
			t.Fatal("Expected users array")
		}
		if len(users) < 1 {
			t.Errorf("SearchUsers(%q) expected at least 1 user, got %d", query, len(users))
		}
	}
}

func TestSearchServers_CaseInsensitive(t *testing.T) {
	_, router, cleanup := setupSearchTestDB(t)
	defer cleanup()

	user := createSearchUser(t, "searchcase2", true)
	createSearchRoom(t, "CaseServer", true, &user.ID)

	// Search with different cases
	testCases := []string{"caseserver", "CASESERVER", "CaseServer"}
	for _, query := range testCases {
		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/search/servers?q=%s", query), nil)
		addAuthCookieSearch(req, user.ID)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("SearchServers(%q) expected status 200, got %d", query, w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		servers, ok := response["servers"].([]interface{})
		if !ok {
			t.Fatal("Expected servers array")
		}
		if len(servers) < 1 {
			t.Errorf("SearchServers(%q) expected at least 1 server, got %d", query, len(servers))
		}
	}
}
