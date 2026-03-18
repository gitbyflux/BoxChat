package services

import (
	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupModerationTestDB initializes a test database for moderation tests
func setupModerationTestDB(t *testing.T) func() {
	// Reset database state for test
	database.ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	return func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
	}
}

// createTestUser creates a test user with the given username
func createTestUser(t *testing.T, username string, isSuperuser bool) *models.User {
	t.Helper()
	
	// Check if user already exists
	var existing models.User
	if err := database.DB.Where("username = ?", username).First(&existing).Error; err == nil {
		return &existing
	}
	
	hashedPassword := hashPassword(t, "password123")
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

// createTestUserWithBan creates a test user with ban status
func createTestUserWithBan(t *testing.T, username string, isSuperuser, isBanned bool) *models.User {
	t.Helper()
	
	// Check if user already exists
	var existing models.User
	if err := database.DB.Where("username = ?", username).First(&existing).Error; err == nil {
		// Update ban status if needed
		existing.IsBanned = isBanned
		database.DB.Save(&existing)
		return &existing
	}
	
	hashedPassword := hashPassword(t, "password123")
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

// createTestRoom creates a test room
func createTestRoom(t *testing.T, name string) *models.Room {
	t.Helper()
	room := models.Room{
		Name:        name,
		Description: "Test room",
		Type:        "server",
	}
	if err := database.DB.Create(&room).Error; err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}
	return &room
}

// createTestMember creates a member in a room
func createTestMember(t *testing.T, userID, roomID uint, role string) *models.Member {
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

// ============================================================================
// ParseDuration Tests
// ============================================================================

func TestParseDuration_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"30 minutes", "30m", 30},
		{"2 hours", "2h", 120},
		{"1 day", "1d", 1440},
		{"5 minutes no unit", "5", 5},
		{"24 hours", "24h", 1440},
		{"7 days", "7d", 10080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDuration(tt.input)
			if err != nil {
				t.Fatalf("ParseDuration(%q) error = %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ParseDuration(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Empty string", ""},
		{"Invalid unit", "5x"},
		{"Negative value", "-5m"},
		{"Zero value", "0m"},
		{"No number", "abc"},
		{"Invalid format", "5mm"},
		{"Just unit", "m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDuration(tt.input)
			if err != ErrInvalidDuration {
				t.Errorf("ParseDuration(%q) error = %v, want %v", tt.input, err, ErrInvalidDuration)
			}
		})
	}
}

// ============================================================================
// FindRoomMemberByToken Tests
// ============================================================================

func TestFindRoomMemberByToken_Success(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	// Create user and room
	user := createTestUser(t, "testuser", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, user.ID, room.ID, "member")

	// Test with @username
	member, err := FindRoomMemberByToken(room.ID, "@testuser")
	if err != nil {
		t.Fatalf("FindRoomMemberByToken() error = %v", err)
	}
	if member.UserID != user.ID {
		t.Errorf("FindRoomMemberByToken() userID = %d, want %d", member.UserID, user.ID)
	}

	// Test without @
	member2, err := FindRoomMemberByToken(room.ID, "testuser")
	if err != nil {
		t.Fatalf("FindRoomMemberByToken() without @ error = %v", err)
	}
	if member2.UserID != user.ID {
		t.Errorf("FindRoomMemberByToken() without @ userID = %d, want %d", member2.UserID, user.ID)
	}
}

func TestFindRoomMemberByToken_CaseInsensitive(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	user := createTestUser(t, "TestUser", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, user.ID, room.ID, "member")

	// Test with different cases
	testCases := []string{"@testuser", "@TESTUSER", "@TeStUsEr", "testuser"}
	for _, token := range testCases {
		t.Run(token, func(t *testing.T) {
			member, err := FindRoomMemberByToken(room.ID, token)
			if err != nil {
				t.Errorf("FindRoomMemberByToken(%q) error = %v", token, err)
			}
			if member.UserID != user.ID {
				t.Errorf("FindRoomMemberByToken(%q) userID = %d, want %d", token, member.UserID, user.ID)
			}
		})
	}
}

func TestFindRoomMemberByToken_NotFound(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")

	_, err := FindRoomMemberByToken(room.ID, "@nonexistent")
	if err != ErrMemberNotFound {
		t.Errorf("FindRoomMemberByToken() error = %v, want %v", err, ErrMemberNotFound)
	}

	// Test empty token
	_, err = FindRoomMemberByToken(room.ID, "@")
	if err != ErrMemberNotFound {
		t.Errorf("FindRoomMemberByToken(@) error = %v, want %v", err, ErrMemberNotFound)
	}
}

// ============================================================================
// CanUserModerate Tests
// ============================================================================

func TestCanUserModerate_Superuser(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	superuser := createTestUser(t, "admin", true)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, superuser.ID, room.ID, "member")

	if !CanUserModerate(superuser.ID, room.ID, "mute_members") {
		t.Error("CanUserModerate() should return true for superuser")
	}
}

func TestCanUserModerate_RoomOwner(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	user := createTestUser(t, "owner", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, user.ID, room.ID, "owner")

	if !CanUserModerate(user.ID, room.ID, "ban_members") {
		t.Error("CanUserModerate() should return true for room owner")
	}
}

func TestCanUserModerate_RoomAdmin(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	user := createTestUser(t, "admin", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, user.ID, room.ID, "admin")

	if !CanUserModerate(user.ID, room.ID, "kick_members") {
		t.Error("CanUserModerate() should return true for room admin")
	}
}

func TestCanUserModerate_NoPermission(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	user := createTestUser(t, "member", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, user.ID, room.ID, "member")

	if CanUserModerate(user.ID, room.ID, "mute_members") {
		t.Error("CanUserModerate() should return false for regular member")
	}
}

func TestCanUserModerate_UserNotFound(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")

	if CanUserModerate(99999, room.ID, "mute_members") {
		t.Error("CanUserModerate() should return false for non-existent user")
	}
}

// ============================================================================
// Mute Tests
// ============================================================================

func TestModerationService_Mute_Success(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	// Create moderator and target
	moderator := createTestUser(t, "moderator", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, muteUpdate, err := service.Mute(moderator.ID, room.ID, "@target", "30m", "spam")

	if err != nil {
		t.Fatalf("Mute() error = %v", err)
	}
	if !result.OK {
		t.Errorf("Mute() result.OK = false, want true")
	}
	if muteUpdate == nil {
		t.Fatal("Mute() muteUpdate = nil")
	}
	if muteUpdate.RoomID != room.ID {
		t.Errorf("Mute() roomID = %d, want %d", muteUpdate.RoomID, room.ID)
	}
	if muteUpdate.UserID != target.ID {
		t.Errorf("Mute() userID = %d, want %d", muteUpdate.UserID, target.ID)
	}
	if muteUpdate.MutedUntil == nil {
		t.Error("Mute() mutedUntil = nil")
	}
}

func TestModerationService_Mute_NoPermission(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "member", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "member")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, _, err := service.Mute(moderator.ID, room.ID, "@target", "30m", "")

	if err != ErrNoPermission {
		t.Errorf("Mute() error = %v, want %v", err, ErrNoPermission)
	}
	if result.OK {
		t.Error("Mute() result.OK = true, want false")
	}
}

func TestModerationService_Mute_SelfMute(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "moderator", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")

	service := NewModerationService()
	result, _, err := service.Mute(moderator.ID, room.ID, "@moderator", "30m", "")

	if err != ErrSelfAction {
		t.Errorf("Mute() error = %v, want %v", err, ErrSelfAction)
	}
	if result.OK {
		t.Error("Mute() result.OK = true, want false")
	}
}

func TestModerationService_Mute_InvalidDuration(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "admin", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, _, err := service.Mute(moderator.ID, room.ID, "@target", "invalid", "")

	if err != ErrInvalidDuration {
		t.Errorf("Mute() error = %v, want %v", err, ErrInvalidDuration)
	}
	if result.OK {
		t.Error("Mute() result.OK = true, want false")
	}
}

func TestModerationService_Mute_UserNotFound(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "admin", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")

	service := NewModerationService()
	result, _, err := service.Mute(moderator.ID, room.ID, "@nonexistent", "30m", "")

	if err != ErrMemberNotFound {
		t.Errorf("Mute() error = %v, want %v", err, ErrMemberNotFound)
	}
	if result.OK {
		t.Error("Mute() result.OK = true, want false")
	}
}

// ============================================================================
// Unmute Tests
// ============================================================================

func TestModerationService_Unmute_Success(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "admin", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, muteUpdate, err := service.Unmute(moderator.ID, room.ID, "@target")

	if err != nil {
		t.Fatalf("Unmute() error = %v", err)
	}
	if !result.OK {
		t.Errorf("Unmute() result.OK = false, want true")
	}
	if muteUpdate == nil {
		t.Fatal("Unmute() muteUpdate = nil")
	}
	if muteUpdate.MutedUntil != nil {
		t.Error("Unmute() mutedUntil should be nil")
	}
}

func TestModerationService_Unmute_NoPermission(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "member", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "member")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, _, err := service.Unmute(moderator.ID, room.ID, "@target")

	if err != ErrNoPermission {
		t.Errorf("Unmute() error = %v, want %v", err, ErrNoPermission)
	}
	if result.OK {
		t.Error("Unmute() result.OK = true, want false")
	}
}

// ============================================================================
// Kick Tests
// ============================================================================

func TestModerationService_Kick_Success(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "admin", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, removed, redirect, err := service.Kick(moderator.ID, room.ID, "@target", "spam")

	if err != nil {
		t.Fatalf("Kick() error = %v", err)
	}
	if !result.OK {
		t.Errorf("Kick() result.OK = false, want true")
	}
	if removed == nil {
		t.Fatal("Kick() removed = nil")
	}
	if removed.UserID != target.ID {
		t.Errorf("Kick() userID = %d, want %d", removed.UserID, target.ID)
	}
	if redirect == nil {
		t.Fatal("Kick() redirect = nil")
	}
	if redirect.Location != "/" {
		t.Errorf("Kick() redirect.Location = %s, want /", redirect.Location)
	}
}

func TestModerationService_Kick_SelfKick(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "moderator", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")

	service := NewModerationService()
	result, _, _, err := service.Kick(moderator.ID, room.ID, "@moderator", "")

	if err != ErrSelfAction {
		t.Errorf("Kick() error = %v, want %v", err, ErrSelfAction)
	}
	if result.OK {
		t.Error("Kick() result.OK = true, want false")
	}
}

// ============================================================================
// Ban Tests
// ============================================================================

func TestModerationService_Ban_Success(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "admin", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, removed, redirect, err := service.Ban(moderator.ID, room.ID, "@target", "2h", "spam")

	if err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	if !result.OK {
		t.Errorf("Ban() result.OK = false, want true")
	}
	if removed == nil {
		t.Fatal("Ban() removed = nil")
	}
	if redirect == nil {
		t.Fatal("Ban() redirect = nil")
	}
}

func TestModerationService_Ban_NoPermission(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "member", false)
	target := createTestUser(t, "target", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "member")
	createTestMember(t, target.ID, room.ID, "member")

	service := NewModerationService()
	result, _, _, err := service.Ban(moderator.ID, room.ID, "@target", "", "")

	if err != ErrNoPermission {
		t.Errorf("Ban() error = %v, want %v", err, ErrNoPermission)
	}
	if result.OK {
		t.Error("Ban() result.OK = true, want false")
	}
}

func TestModerationService_Ban_SelfBan(t *testing.T) {
	cleanup := setupModerationTestDB(t)
	defer cleanup()

	moderator := createTestUser(t, "moderator", false)
	room := createTestRoom(t, "Test Room")
	createTestMember(t, moderator.ID, room.ID, "admin")

	service := NewModerationService()
	result, _, _, err := service.Ban(moderator.ID, room.ID, "@moderator", "", "")

	if err != ErrSelfAction {
		t.Errorf("Ban() error = %v, want %v", err, ErrSelfAction)
	}
	if result.OK {
		t.Error("Ban() result.OK = true, want false")
	}
}

// ============================================================================
// CommandResult Tests
// ============================================================================

func TestCommandResult_Success(t *testing.T) {
	result := CommandResult{
		OK:      true,
		Message: "Success",
	}

	if !result.OK {
		t.Error("CommandResult.OK should be true")
	}
	if result.Message != "Success" {
		t.Errorf("CommandResult.Message = %s, want Success", result.Message)
	}
}

func TestCommandResult_Error(t *testing.T) {
	result := CommandResult{
		OK:      false,
		Message: "Permission denied",
	}

	if result.OK {
		t.Error("CommandResult.OK should be false")
	}
	if result.Message != "Permission denied" {
		t.Errorf("CommandResult.Message = %s, want 'Permission denied'", result.Message)
	}
}

// ============================================================================
// MemberMuteUpdate Tests
// ============================================================================

func TestMemberMuteUpdate(t *testing.T) {
	future := time.Now().Add(30 * time.Minute)
	update := MemberMuteUpdate{
		RoomID:     1,
		UserID:     2,
		MutedUntil: &future,
	}

	if update.RoomID != 1 {
		t.Errorf("MemberMuteUpdate.RoomID = %d, want 1", update.RoomID)
	}
	if update.UserID != 2 {
		t.Errorf("MemberMuteUpdate.UserID = %d, want 2", update.UserID)
	}
	if update.MutedUntil == nil {
		t.Error("MemberMuteUpdate.MutedUntil should not be nil")
	}
}

// ============================================================================
// MemberRemoved Tests
// ============================================================================

func TestMemberRemoved(t *testing.T) {
	removed := MemberRemoved{
		UserID: 1,
		RoomID: 2,
	}

	if removed.UserID != 1 {
		t.Errorf("MemberRemoved.UserID = %d, want 1", removed.UserID)
	}
	if removed.RoomID != 2 {
		t.Errorf("MemberRemoved.RoomID = %d, want 2", removed.RoomID)
	}
}

// ============================================================================
// ForceRedirect Tests
// ============================================================================

func TestForceRedirect(t *testing.T) {
	redirect := ForceRedirect{
		Location: "/",
		Reason:   "You were kicked",
	}

	if redirect.Location != "/" {
		t.Errorf("ForceRedirect.Location = %s, want /", redirect.Location)
	}
	if redirect.Reason != "You were kicked" {
		t.Errorf("ForceRedirect.Reason = %s, want 'You were kicked'", redirect.Reason)
	}
}
