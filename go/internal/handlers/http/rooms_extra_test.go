package http

import (
	"boxchat/internal/testutil"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Test Setup Helpers for Rooms Extra
// ============================================================================

func setupRoomsExtraTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	cfg, dbCleanup := testutil.SetupTestDB(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	authHandler := NewAuthHandler(cfg)
	apiHandler := NewAPIHandler(cfg)

	authHandler.RegisterRoutes(router)
	apiHandler.RegisterRoutes(router)
	apiHandler.RegisterRoomsExtraRoutes(router)

	return cfg, router, dbCleanup
}

// ============================================================================
// CreateRoomInvite Tests
// ============================================================================

func TestCreateRoomInvite_Owner(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "inviteowner", false)
	room := createTestRoomForAPI(t, "Invite Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "owner")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/invite", room.ID), nil)
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
	if response["invite_token"] == "" {
		t.Error("Expected invite_token to be set")
	}
	if response["invite_link"] == "" {
		t.Error("Expected invite_link to be set")
	}
}

func TestCreateRoomInvite_Admin(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "inviteadmin", false)
	room := createTestRoomForAPI(t, "Admin Invite Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "admin")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/invite", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCreateRoomInvite_NoPermission(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "noinvitepermuser", false)
	otherUser := createTestUserForAPI(t, "noinviteotheruser", false)
	room := createTestRoomForAPI(t, "No Invite Perm Room", "server", otherUser.ID)
	createTestMemberForAPI(t, otherUser.ID, room.ID, "owner")
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/invite", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestCreateRoomInvite_NotMember(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notmemberinviteuser", false)
	otherUser := createTestUserForAPI(t, "notmemberinviteotheruser", false)
	room := createTestRoomForAPI(t, "Not Member Invite Room", "server", otherUser.ID)
	createTestMemberForAPI(t, otherUser.ID, room.ID, "owner")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/invite", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// JoinRoomByInvite Tests
// ============================================================================

func TestJoinRoomByInvite_Success(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	owner := createTestUserForAPI(t, "invitetokenowner", false)
	user := createTestUserForAPI(t, "joinbyinviteuser", false)
	room := createTestRoomForAPI(t, "Join By Invite Room", "server", owner.ID)
	room.InviteToken = "test_invite_token_123"
	database.DB.Save(room)
	createTestMemberForAPI(t, owner.ID, room.ID, "owner")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/join/%s", room.InviteToken), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify user became a member
	var member models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", user.ID, room.ID).First(&member)
	if result.Error != nil {
		t.Error("Expected user to become a member")
	}
}

func TestJoinRoomByInvite_InvalidToken(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "invalidtokenuser", false)

	req, _ := http.NewRequest("GET", "/api/v1/join/invalid_token", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestJoinRoomByInvite_AlreadyMember(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	owner := createTestUserForAPI(t, "alreadymemberowner", false)
	user := createTestUserForAPI(t, "alreadymemberuser", false)
	room := createTestRoomForAPI(t, "Already Member Room", "server", owner.ID)
	room.InviteToken = "already_member_token"
	database.DB.Save(room)
	createTestMemberForAPI(t, owner.ID, room.ID, "owner")
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/join/%s", room.InviteToken), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// LeaveRoom Tests
// ============================================================================

func TestLeaveRoom_Success(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "leaveroomuser", false)
	room := createTestRoomForAPI(t, "Leave Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/leave", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify membership was deleted
	var member models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", user.ID, room.ID).First(&member)
	if result.Error == nil {
		t.Error("Expected membership to be deleted")
	}
}

func TestLeaveRoom_OwnerCannotLeave(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "ownerleaveuser", false)
	room := createTestRoomForAPI(t, " Owner Leave Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "owner")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/leave", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestLeaveRoom_NotMember(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notmemberleaveuser", false)
	otherUser := createTestUserForAPI(t, "notmemberleaveotheruser", false)
	room := createTestRoomForAPI(t, "Not Member Leave Room", "server", otherUser.ID)
	createTestMemberForAPI(t, otherUser.ID, room.ID, "owner")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/leave", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// DeleteDM Tests
// ============================================================================

func TestDeleteDM_Success(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user1 := createTestUserForAPI(t, "deletedmuser1", false)
	user2 := createTestUserForAPI(t, "deletedmuser2", false)
	dmRoom := models.Room{
		Name: "DM Room",
		Type: "dm",
	}
	database.DB.Create(&dmRoom)
	createTestMemberForAPI(t, user1.ID, dmRoom.ID, "owner")
	createTestMemberForAPI(t, user2.ID, dmRoom.ID, "member")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/delete_dm", dmRoom.ID), nil)
	addAuthCookieForAPI(req, user1.ID, user1.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify user1's membership was deleted (but room still exists as user2 is still member)
	var member1 models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", user1.ID, dmRoom.ID).First(&member1)
	if result.Error == nil {
		t.Error("Expected user1's membership to be deleted")
	}

	// Verify room still exists (user2 is still a member)
	var remainingRoom models.Room
	result = database.DB.First(&remainingRoom, dmRoom.ID)
	if result.Error != nil {
		t.Error("Expected DM room to still exist (user2 is still member)")
	}

	// Verify user2's membership still exists
	var member2 models.Member
	result = database.DB.Where("user_id = ? AND room_id = ?", user2.ID, dmRoom.ID).First(&member2)
	if result.Error != nil {
		t.Error("Expected user2's membership to still exist")
	}
}

func TestDeleteDM_NotDMRoom(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notdmuser", false)
	room := createTestRoomForAPI(t, "Not DM Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "owner")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/delete_dm", room.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDeleteDM_NotMember(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notmemberdeleteuser", false)
	otherUser := createTestUserForAPI(t, "notmemberdeleteotheruser", false)
	dmRoom := models.Room{
		Name: "Not Member DM",
		Type: "dm",
	}
	database.DB.Create(&dmRoom)
	createTestMemberForAPI(t, otherUser.ID, dmRoom.ID, "owner")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/room/%d/delete_dm", dmRoom.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// GetUserProfile Tests
// ============================================================================

func TestGetUserProfile_Success(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	viewer := createTestUserForAPI(t, "viewprofileuser", false)
	target := createTestUserForAPI(t, "targetprofileuser", false)
	target.Bio = "Test bio"
	target.AvatarURL = "/uploads/avatars/target.jpg"
	database.DB.Save(target)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/user/%d/profile", target.ID), nil)
	addAuthCookieForAPI(req, viewer.ID, viewer.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	profile, ok := response["profile"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected profile object in response")
	}

	if profile["id"] != float64(target.ID) {
		t.Errorf("Expected id=%d, got %v", target.ID, profile["id"])
	}
	if profile["username"] != "targetprofileuser" {
		t.Errorf("Expected username='targetprofileuser', got %v", profile["username"])
	}
	if profile["bio"] != "Test bio" {
		t.Errorf("Expected bio='Test bio', got %v", profile["bio"])
	}
}

func TestGetUserProfile_NotFound(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "notfoundprofileuser", false)

	req, _ := http.NewRequest("GET", "/api/v1/user/999/profile", nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetUserProfile_OwnProfile(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	user := createTestUserForAPI(t, "ownprofileuser", false)
	user.Bio = "My bio"
	database.DB.Save(user)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/user/%d/profile", user.ID), nil)
	addAuthCookieForAPI(req, user.ID, user.Username)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// GetStatistics Tests
// ============================================================================

func TestGetStatistics_Success(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	// Statistics endpoint requires superuser
	user := createTestUserForAPI(t, "statsuser", true)

	// Create some data
	room := createTestRoomForAPI(t, "Stats Room", "server", user.ID)
	createTestMemberForAPI(t, user.ID, room.ID, "member")
	channel := createTestChannelForAPI(t, "general", room.ID)
	createTestMessageForAPI(t, "Message 1", user.ID, channel.ID, "text")
	createTestMessageForAPI(t, "Message 2", user.ID, channel.ID, "text")

	req, _ := http.NewRequest("GET", "/api/v1/statistics", nil)
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

	// Statistics are returned nested under "statistics" key
	stats, ok := response["statistics"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected statistics object in response")
	}

	// Verify statistics are returned
	if stats["total_users"] == nil {
		t.Error("Expected total_users in response")
	}
	if stats["total_rooms"] == nil {
		t.Error("Expected total_rooms in response")
	}
	if stats["total_messages"] == nil {
		t.Error("Expected total_messages in response")
	}
}

func TestGetStatistics_Empty(t *testing.T) {
	_, router, cleanup := setupRoomsExtraTestDB(t)
	defer cleanup()

	// Statistics endpoint requires superuser
	user := createTestUserForAPI(t, "emptystatsuser", true)

	req, _ := http.NewRequest("GET", "/api/v1/statistics", nil)
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

	// Statistics should still be present (with zero values)
	stats, ok := response["statistics"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected statistics object in response")
	}

	// Statistics should be present even if empty/zero
	if stats["total_users"] == nil {
		t.Error("Expected total_users in response")
	}
}
