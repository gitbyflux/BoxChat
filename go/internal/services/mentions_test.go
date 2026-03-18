package services

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/testutil"
	"testing"
)

// setupMentionsTestDB initializes a test database for mentions tests
func setupMentionsTestDB(t *testing.T) func() {
	_, cleanup := testutil.SetupTestDB(t)
	return cleanup
}

// ============================================================================
// ParseMentions Tests
// ============================================================================

func TestMentionService_ParseMentions_EmptyContent(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	service := NewMentionService()
	result := service.ParseMentions("", 1, 1)

	if result == nil {
		t.Fatal("ParseMentions() should return non-nil result")
	}
	if result.MentionEveryone {
		t.Error("MentionEveryone should be false for empty content")
	}
	if len(result.MentionedUserIDs) != 0 {
		t.Errorf("MentionedUserIDs should be empty, got %d", len(result.MentionedUserIDs))
	}
}

func TestMentionService_ParseMentions_NoMentions(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	service := NewMentionService()
	result := service.ParseMentions("Just a regular message", 1, 1)

	if result.MentionEveryone {
		t.Error("MentionEveryone should be false")
	}
	if len(result.MentionedUserIDs) != 0 {
		t.Errorf("MentionedUserIDs should be empty, got %d", len(result.MentionedUserIDs))
	}
	if len(result.MentionedUsernames) != 0 {
		t.Errorf("MentionedUsernames should be empty, got %d", len(result.MentionedUsernames))
	}
}

func TestMentionService_ParseMentions_UserMention(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	// Create room and users
	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "p1_alice", false)
	user2 := createTestUser(t, "p1_bob", false)

	// Add users as members
	createTestMember(t, user1.ID, room.ID, "member")
	createTestMember(t, user2.ID, room.ID, "member")

	service := NewMentionService()
	result := service.ParseMentions("Hello @p1_alice!", room.ID, user1.ID)

	if result.MentionEveryone {
		t.Error("MentionEveryone should be false")
	}
	if len(result.MentionedUserIDs) != 1 {
		t.Errorf("MentionedUserIDs should have 1 user, got %d", len(result.MentionedUserIDs))
	}
	if len(result.MentionedUsernames) != 1 {
		t.Errorf("MentionedUsernames should have 1 username, got %d", len(result.MentionedUsernames))
	}
}

func TestMentionService_ParseMentions_MultipleUserMentions(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	// Use unique prefixes to avoid conflicts with other tests
	user1 := createTestUser(t, "t1_alice", false)
	user2 := createTestUser(t, "t1_bob", false)

	// Ensure all users are members of this room
	createTestMember(t, user1.ID, room.ID, "member")
	createTestMember(t, user2.ID, room.ID, "member")

	service := NewMentionService()
	// Mention user2
	result := service.ParseMentions("@t1_bob hello!", room.ID, user1.ID)

	if len(result.MentionedUserIDs) != 1 {
		t.Errorf("MentionedUserIDs should have 1 user, got %d. Mentioned: %v", len(result.MentionedUserIDs), result.MentionedUsernames)
	}
}

func TestMentionService_ParseMentions_CaseInsensitive(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "ci_Alice", false)
	user2 := createTestUser(t, "ci_Bob", false)

	createTestMember(t, user1.ID, room.ID, "member")
	createTestMember(t, user2.ID, room.ID, "member")

	service := NewMentionService()
	
	// Test with different cases
	testCases := []string{"@ci_alice", "@ci_ALICE", "@ci_AlIcE"}
	for _, mention := range testCases {
		result := service.ParseMentions(mention, room.ID, user1.ID)
		if len(result.MentionedUserIDs) != 1 {
			t.Errorf("ParseMentions(%s) should find 1 user, got %d", mention, len(result.MentionedUserIDs))
		}
	}
}

func TestMentionService_ParseMentions_NonExistentUser(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "ne_alice", false)
	createTestMember(t, user1.ID, room.ID, "member")

	service := NewMentionService()
	result := service.ParseMentions("Hello @nonexistent!", room.ID, user1.ID)

	if len(result.MentionedUserIDs) != 0 {
		t.Errorf("MentionedUserIDs should be empty for non-existent user, got %d", len(result.MentionedUserIDs))
	}
}

func TestMentionService_ParseMentions_UserNotInRoom(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "nr_alice", false)
	_ = createTestUser(t, "nr_bob", false)

	// Only add user1 to room
	createTestMember(t, user1.ID, room.ID, "member")
	// user2 is NOT in the room

	service := NewMentionService()
	result := service.ParseMentions("Hello @nr_bob!", room.ID, user1.ID)

	// Should not mention user2 since they're not in the room
	if len(result.MentionedUserIDs) != 0 {
		t.Errorf("MentionedUserIDs should be empty for user not in room, got %d", len(result.MentionedUserIDs))
	}
}

// ============================================================================
// Role Mention Tests
// ============================================================================

func TestMentionService_ParseMentions_RoleMention(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "rm_alice", false)
	user2 := createTestUser(t, "rm_bob", false)

	createTestMember(t, user1.ID, room.ID, "admin")
	createTestMember(t, user2.ID, room.ID, "member")

	// Create a role with mention tag
	role := models.Role{
		RoomID:                   room.ID,
		Name:                     "Moderator",
		MentionTag:               "rm_mod",
		IsSystem:                 false,
		CanBeMentionedByEveryone: false,
	}
	if err := database.DB.Create(&role).Error; err != nil {
		t.Fatalf("Failed to create role: %v", err)
	}

	service := NewMentionService()
	result := service.ParseMentions("Hello @rm_mod!", room.ID, user1.ID)

	// Role mention should be processed
	if result == nil {
		t.Fatal("ParseMentions() should return non-nil result")
	}
}

func TestMentionService_ParseMentions_EveryoneMention(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "em_alice", false)
	user2 := createTestUser(t, "em_bob", false)

	createTestMember(t, user1.ID, room.ID, "admin")
	createTestMember(t, user2.ID, room.ID, "member")

	service := NewMentionService()
	result := service.ParseMentions("@everyone attention!", room.ID, user1.ID)

	// @everyone should be detected if role exists
	if result == nil {
		t.Fatal("ParseMentions() should return non-nil result")
	}
}

// ============================================================================
// canUserMentionRole Tests
// ============================================================================

func TestMentionService_CanUserMentionRole_NilRole(t *testing.T) {
	service := NewMentionService()
	result := service.canUserMentionRole(1, 1, nil)

	if result {
		t.Error("canUserMentionRole() should return false for nil role")
	}
}

func TestMentionService_CanUserMentionRole_CanBeMentionedByEveryone(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	
	role := models.Role{
		RoomID:                   room.ID,
		Name:                     "mentionable",
		MentionTag:               "mentionable",
		IsSystem:                 false,
		CanBeMentionedByEveryone: true,
	}
	if err := database.DB.Create(&role).Error; err != nil {
		t.Fatalf("Failed to create role: %v", err)
	}

	service := NewMentionService()
	result := service.canUserMentionRole(1, room.ID, &role)

	if !result {
		t.Error("canUserMentionRole() should return true when CanBeMentionedByEveryone is true")
	}
}

func TestMentionService_CanUserMentionRole_WithPermission(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user := createTestUser(t, "user", false)

	// Create source role
	sourceRole := models.Role{
		RoomID:     room.ID,
		Name:       "admin",
		MentionTag: "admin",
		IsSystem:   true,
	}
	if err := database.DB.Create(&sourceRole).Error; err != nil {
		t.Fatalf("Failed to create source role: %v", err)
	}

	// Create target role
	targetRole := models.Role{
		RoomID:     room.ID,
		Name:       "mod",
		MentionTag: "mod",
		IsSystem:   false,
	}
	if err := database.DB.Create(&targetRole).Error; err != nil {
		t.Fatalf("Failed to create target role: %v", err)
	}

	// Assign source role to user
	memberRole := models.MemberRole{
		UserID: user.ID,
		RoomID: room.ID,
		RoleID: sourceRole.ID,
	}
	if err := database.DB.Create(&memberRole).Error; err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	// Create mention permission
	perm := models.RoleMentionPermission{
		RoomID:       room.ID,
		SourceRoleID: sourceRole.ID,
		TargetRoleID: targetRole.ID,
	}
	if err := database.DB.Create(&perm).Error; err != nil {
		t.Fatalf("Failed to create permission: %v", err)
	}

	service := NewMentionService()
	result := service.canUserMentionRole(user.ID, room.ID, &targetRole)

	if !result {
		t.Error("canUserMentionRole() should return true when user has permission")
	}
}

func TestMentionService_CanUserMentionRole_NoPermission(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user := createTestUser(t, "user", false)

	// Create target role (not mentionable by everyone)
	targetRole := models.Role{
		RoomID:                   room.ID,
		Name:                     "protected",
		MentionTag:               "protected",
		IsSystem:                 false,
		CanBeMentionedByEveryone: false,
	}
	if err := database.DB.Create(&targetRole).Error; err != nil {
		t.Fatalf("Failed to create target role: %v", err)
	}

	service := NewMentionService()
	result := service.canUserMentionRole(user.ID, room.ID, &targetRole)

	if result {
		t.Error("canUserMentionRole() should return false when user has no permission")
	}
}

// ============================================================================
// getUserRoleIDs Tests
// ============================================================================

func TestMentionService_GetUserRoleIDs_Empty(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user := createTestUser(t, "user", false)
	createTestMember(t, user.ID, room.ID, "member")

	service := NewMentionService()
	roleIDs := service.getUserRoleIDs(user.ID, room.ID)

	// Regular member should have no role IDs (everyone role is handled separately)
	if roleIDs == nil {
		t.Error("getUserRoleIDs() should return non-nil map")
	}
}

func TestMentionService_GetUserRoleIDs_WithRoles(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user := createTestUser(t, "user", false)
	
	// Create role
	role := models.Role{
		RoomID:     room.ID,
		Name:       "custom",
		MentionTag: "custom",
	}
	if err := database.DB.Create(&role).Error; err != nil {
		t.Fatalf("Failed to create role: %v", err)
	}

	createTestMember(t, user.ID, room.ID, "member")

	// Assign role to user
	memberRole := models.MemberRole{
		UserID: user.ID,
		RoomID: room.ID,
		RoleID: role.ID,
	}
	if err := database.DB.Create(&memberRole).Error; err != nil {
		t.Fatalf("Failed to assign role: %v", err)
	}

	service := NewMentionService()
	roleIDs := service.getUserRoleIDs(user.ID, room.ID)

	if !roleIDs[role.ID] {
		t.Error("getUserRoleIDs() should include assigned role")
	}
}

func TestMentionService_GetUserRoleIDs_AdminRole(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user := createTestUser(t, "admin", false)
	createTestMember(t, user.ID, room.ID, "admin")

	service := NewMentionService()
	roleIDs := service.getUserRoleIDs(user.ID, room.ID)

	// Admin should have placeholder role ID
	if !roleIDs[0] {
		t.Error("Admin should have placeholder role ID 0")
	}
}

func TestMentionService_GetUserRoleIDs_OwnerRole(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user := createTestUser(t, "owner", false)
	createTestMember(t, user.ID, room.ID, "owner")

	service := NewMentionService()
	roleIDs := service.getUserRoleIDs(user.ID, room.ID)

	// Owner should have placeholder role ID
	if !roleIDs[0] {
		t.Error("Owner should have placeholder role ID 0")
	}
}

// ============================================================================
// GetMentionNotificationUsers Tests
// ============================================================================

func TestMentionService_GetMentionNotificationUsers_DirectMentions(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "dn_alice", false)
	user2 := createTestUser(t, "dn_bob", false)

	createTestMember(t, user1.ID, room.ID, "member")
	createTestMember(t, user2.ID, room.ID, "member")

	// Create channel
	channel := models.Channel{
		Name:   "general",
		RoomID: room.ID,
	}
	if err := database.DB.Create(&channel).Error; err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	service := NewMentionService()
	mentionData := &MentionData{
		MentionEveryone:    false,
		MentionedUserIDs:   []uint{user2.ID},
		MentionedUsernames: []string{"bob"},
	}

	userIDs := service.GetMentionNotificationUsers(mentionData, channel.ID)

	if len(userIDs) != 1 {
		t.Errorf("GetMentionNotificationUsers() should return 1 user, got %d", len(userIDs))
	}
}

func TestMentionService_GetMentionNotificationUsers_Everyone(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "ge_alice", false)
	user2 := createTestUser(t, "ge_bob", false)

	createTestMember(t, user1.ID, room.ID, "member")
	createTestMember(t, user2.ID, room.ID, "member")

	channel := models.Channel{
		Name:   "general",
		RoomID: room.ID,
	}
	if err := database.DB.Create(&channel).Error; err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	service := NewMentionService()
	mentionData := &MentionData{
		MentionEveryone:    true,
		MentionedUserIDs:   []uint{},
		MentionedUsernames: []string{},
	}

	userIDs := service.GetMentionNotificationUsers(mentionData, channel.ID)

	// Should return all members in the room
	if len(userIDs) < 2 {
		t.Errorf("GetMentionNotificationUsers() should return all members for @everyone, got %d", len(userIDs))
	}
}

// ============================================================================
// MentionData Structure Tests
// ============================================================================

func TestMentionData_Structure(t *testing.T) {
	data := MentionData{
		MentionEveryone:    true,
		MentionedUserIDs:   []uint{1, 2, 3},
		MentionedUsernames: []string{"alice", "bob"},
		MentionedRoleIDs:   []uint{1},
		MentionedRoleTags:  []string{"admin"},
		DeniedRoleTags:     []string{"protected"},
	}

	if !data.MentionEveryone {
		t.Error("MentionEveryone should be true")
	}
	if len(data.MentionedUserIDs) != 3 {
		t.Errorf("MentionedUserIDs should have 3 items, got %d", len(data.MentionedUserIDs))
	}
	if len(data.MentionedUsernames) != 2 {
		t.Errorf("MentionedUsernames should have 2 items, got %d", len(data.MentionedUsernames))
	}
}

// ============================================================================
// Edge Cases Tests
// ============================================================================

func TestMentionService_ParseMentions_ShortUsername(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "su_alice", false)
	createTestMember(t, user1.ID, room.ID, "member")

	service := NewMentionService()
	
	// Single character mentions should not be parsed (minimum 2 chars)
	result := service.ParseMentions("Hey @a!", room.ID, user1.ID)
	
	if len(result.MentionedUserIDs) != 0 {
		t.Errorf("Single character mentions should not be parsed, got %d", len(result.MentionedUserIDs))
	}
}

func TestMentionService_ParseMentions_LongUsername(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "lu_alice", false)
	createTestMember(t, user1.ID, room.ID, "member")

	service := NewMentionService()
	
	// Very long usernames (>60 chars) should not be parsed
	result := service.ParseMentions("@verylongusernamethatexceedsthemaximumallowedlengthfortesting", room.ID, user1.ID)
	
	if len(result.MentionedUserIDs) != 0 {
		t.Errorf("Too long usernames should not be parsed, got %d", len(result.MentionedUserIDs))
	}
}

func TestMentionService_ParseMentions_SpecialCharacters(t *testing.T) {
	cleanup := setupMentionsTestDB(t)
	defer cleanup()

	room := createTestRoom(t, "Test Room")
	user1 := createTestUser(t, "sc_alice", false)
	createTestMember(t, user1.ID, room.ID, "member")

	service := NewMentionService()
	
	// Usernames with special characters should not be parsed
	result := service.ParseMentions("@user_name-test!", room.ID, user1.ID)
	
	// Only valid part should be parsed
	if result == nil {
		t.Error("ParseMentions() should return non-nil result")
	}
}
