package services

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/testutil"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Helper Functions for Role Service Tests
// ============================================================================

func setupRoleServiceTestDB(t *testing.T) func() {
	cfg, cleanup := testutil.SetupTestDB(t)
	_ = cfg // Use config if needed
	return cleanup
}

func createRoleServiceUser(t *testing.T, username string, isSuperuser bool) *models.User {
	t.Helper()
	hashedPassword := hashPasswordRoleService(t, "password123")
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

func createRoleServiceRoom(t *testing.T, ownerID *uint) *models.Room {
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

func createRoleServiceMember(t *testing.T, userID, roomID uint, role string) {
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

func createRoleServiceRole(t *testing.T, roomID uint, name, tag string, permissions []string) *models.Role {
	t.Helper()
	role := models.Role{
		RoomID:                   roomID,
		Name:                     name,
		MentionTag:               tag,
		IsSystem:                 false,
		CanBeMentionedByEveryone: false,
	}
	
	if len(permissions) > 0 {
		service := NewRoleService()
		err := service.SetRolePermissions(&role, permissions)
		if err != nil {
			t.Fatalf("Failed to set role permissions: %v", err)
		}
	} else {
		role.PermissionsJSON = "[]"
		database.DB.Create(&role)
	}
	
	return &role
}

func hashPasswordRoleService(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

// ============================================================================
// NewRoleService Tests
// ============================================================================

func TestNewRoleService(t *testing.T) {
	service := NewRoleService()
	if service == nil {
		t.Error("NewRoleService() should return non-nil service")
	}
}

// ============================================================================
// IsValidPermission Tests
// ============================================================================

func TestRoleService_IsValidPermission_ValidPermissions(t *testing.T) {
	service := NewRoleService()
	
	validPermissions := []string{
		"manage_server",
		"manage_roles",
		"manage_channels",
		"invite_members",
		"delete_server",
		"delete_messages",
		"kick_members",
		"ban_members",
		"mute_members",
	}
	
	for _, perm := range validPermissions {
		if !service.IsValidPermission(perm) {
			t.Errorf("IsValidPermission(%q) should return true", perm)
		}
	}
}

func TestRoleService_IsValidPermission_InvalidPermissions(t *testing.T) {
	service := NewRoleService()
	
	invalidPermissions := []string{
		"invalid_permission",
		"admin",
		"",
		"MANAGE_SERVER",
	}
	
	for _, perm := range invalidPermissions {
		if service.IsValidPermission(perm) {
			t.Errorf("IsValidPermission(%q) should return false", perm)
		}
	}
}

// ============================================================================
// GetRolePermissions Tests
// ============================================================================

func TestRoleService_GetRolePermissions_Empty(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	service := NewRoleService()
	role := &models.Role{
		PermissionsJSON: "",
	}
	
	permissions := service.GetRolePermissions(role)
	if len(permissions) != 0 {
		t.Errorf("GetRolePermissions() should return empty slice, got %d permissions", len(permissions))
	}
}

func TestRoleService_GetRolePermissions_WithPermissions(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	service := NewRoleService()
	role := &models.Role{
		PermissionsJSON: `["manage_server", "ban_members"]`,
	}
	
	permissions := service.GetRolePermissions(role)
	if len(permissions) != 2 {
		t.Errorf("GetRolePermissions() should return 2 permissions, got %d", len(permissions))
	}
	
	expectedPerms := map[string]bool{
		"manage_server": true,
		"ban_members":   true,
	}
	
	for _, perm := range permissions {
		if !expectedPerms[perm] {
			t.Errorf("GetRolePermissions() unexpected permission: %s", perm)
		}
	}
}

func TestRoleService_GetRolePermissions_InvalidJSON(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	service := NewRoleService()
	role := &models.Role{
		PermissionsJSON: "invalid json",
	}
	
	permissions := service.GetRolePermissions(role)
	if len(permissions) != 0 {
		t.Errorf("GetRolePermissions() should return empty slice for invalid JSON")
	}
}

// ============================================================================
// SetRolePermissions Tests
// ============================================================================

func TestRoleService_SetRolePermissions_Success(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "roleuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	role := createRoleServiceRole(t, room.ID, "Test Role", "test", []string{})
	
	service := NewRoleService()
	permissions := []string{"manage_server", "ban_members"}
	
	err := service.SetRolePermissions(role, permissions)
	if err != nil {
		t.Errorf("SetRolePermissions() error = %v", err)
	}
	
	// Verify permissions
	var updatedRole models.Role
	database.DB.First(&updatedRole, role.ID)
	
	updatedPerms := service.GetRolePermissions(&updatedRole)
	if len(updatedPerms) != 2 {
		t.Errorf("SetRolePermissions() should set 2 permissions, got %d", len(updatedPerms))
	}
}

func TestRoleService_SetRolePermissions_InvalidPermissions(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "roleuser2", false)
	room := createRoleServiceRoom(t, &user.ID)
	role := createRoleServiceRole(t, room.ID, "Test Role 2", "test2", []string{})
	
	service := NewRoleService()
	permissions := []string{"manage_server", "invalid_permission"}
	
	err := service.SetRolePermissions(role, permissions)
	if err != nil {
		t.Errorf("SetRolePermissions() error = %v", err)
	}
	
	// Verify only valid permission was set
	var updatedRole models.Role
	database.DB.First(&updatedRole, role.ID)
	
	updatedPerms := service.GetRolePermissions(&updatedRole)
	if len(updatedPerms) != 1 {
		t.Errorf("SetRolePermissions() should set only 1 valid permission, got %d", len(updatedPerms))
	}
	if updatedPerms[0] != "manage_server" {
		t.Errorf("SetRolePermissions() should only keep valid permission")
	}
}

// ============================================================================
// GetUserPermissions Tests
// ============================================================================

func TestRoleService_GetUserPermissions_Superuser(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	admin := createRoleServiceUser(t, "superadmin", true)
	room := createRoleServiceRoom(t, nil)
	
	service := NewRoleService()
	permissions := service.GetUserPermissions(admin.ID, room.ID)
	
	// Superuser should have all permissions
	if len(permissions) != len(RolePermissionKeys) {
		t.Errorf("GetUserPermissions() superuser should have all %d permissions, got %d", len(RolePermissionKeys), len(permissions))
	}
	
	for _, key := range RolePermissionKeys {
		if !permissions[key] {
			t.Errorf("GetUserPermissions() superuser should have permission: %s", key)
		}
	}
}

func TestRoleService_GetUserPermissions_RoomOwner(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "roomowneruser", false)
	room := createRoleServiceRoom(t, &user.ID)
	createRoleServiceMember(t, user.ID, room.ID, "owner")
	
	service := NewRoleService()
	permissions := service.GetUserPermissions(user.ID, room.ID)
	
	// Room owner should have all permissions
	if len(permissions) != len(RolePermissionKeys) {
		t.Errorf("GetUserPermissions() room owner should have all %d permissions, got %d", len(RolePermissionKeys), len(permissions))
	}
}

func TestRoleService_GetUserPermissions_RoomAdmin(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "roomadminuser", false)
	room := createRoleServiceRoom(t, nil)
	createRoleServiceMember(t, user.ID, room.ID, "admin")
	
	service := NewRoleService()
	permissions := service.GetUserPermissions(user.ID, room.ID)
	
	// Room admin should have all permissions
	if len(permissions) != len(RolePermissionKeys) {
		t.Errorf("GetUserPermissions() room admin should have all %d permissions, got %d", len(RolePermissionKeys), len(permissions))
	}
}

func TestRoleService_GetUserPermissions_RegularMember(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "memberuser", false)
	room := createRoleServiceRoom(t, nil)
	createRoleServiceMember(t, user.ID, room.ID, "member")
	
	// Create role with some permissions
	role := createRoleServiceRole(t, room.ID, "Member Role", "memberrole", []string{"invite_members"})
	
	// Assign role to user
	memberRole := models.MemberRole{
		UserID: user.ID,
		RoomID: room.ID,
		RoleID: role.ID,
	}
	database.DB.Create(&memberRole)
	
	service := NewRoleService()
	permissions := service.GetUserPermissions(user.ID, room.ID)
	
	// Should have invite_members permission
	if !permissions["invite_members"] {
		t.Error("GetUserPermissions() should have invite_members permission")
	}
}

// ============================================================================
// UserHasPermission Tests
// ============================================================================

func TestRoleService_UserHasPermission_InvalidPermission(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "testuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	service := NewRoleService()
	hasPerm := service.UserHasPermission(user.ID, room.ID, "invalid_permission")
	
	if hasPerm {
		t.Error("UserHasPermission() should return false for invalid permission")
	}
}

func TestRoleService_UserHasPermission_Superuser(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	admin := createRoleServiceUser(t, "superadminuser", true)
	room := createRoleServiceRoom(t, nil)
	
	service := NewRoleService()
	hasPerm := service.UserHasPermission(admin.ID, room.ID, "manage_server")
	
	if !hasPerm {
		t.Error("UserHasPermission() superuser should have manage_server permission")
	}
}

func TestRoleService_UserHasPermission_RoomOwner(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "owneruser", false)
	room := createRoleServiceRoom(t, &user.ID)
	createRoleServiceMember(t, user.ID, room.ID, "owner")
	
	service := NewRoleService()
	hasPerm := service.UserHasPermission(user.ID, room.ID, "ban_members")
	
	if !hasPerm {
		t.Error("UserHasPermission() room owner should have ban_members permission")
	}
}

// ============================================================================
// EnsureDefaultRoles Tests
// ============================================================================

func TestRoleService_EnsureDefaultRoles_CreatesRoles(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "defaultroleuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	service := NewRoleService()
	everyone, admin, err := service.EnsureDefaultRoles(room.ID)
	
	if err != nil {
		t.Errorf("EnsureDefaultRoles() error = %v", err)
	}
	
	if everyone == nil {
		t.Error("EnsureDefaultRoles() should return everyone role")
	}
	
	if admin == nil {
		t.Error("EnsureDefaultRoles() should return admin role")
	}
	
	if everyone.Name != "everyone" {
		t.Errorf("EnsureDefaultRoles() everyone role name = %s, want everyone", everyone.Name)
	}
	
	if admin.Name != "admin" {
		t.Errorf("EnsureDefaultRoles() admin role name = %s, want admin", admin.Name)
	}
}

func TestRoleService_EnsureDefaultRoles_ExistingRoles(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "existingroleuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	service := NewRoleService()
	// Create roles first time
	service.EnsureDefaultRoles(room.ID)
	
	// Create roles second time - should return existing
	everyone2, admin2, err := service.EnsureDefaultRoles(room.ID)
	
	if err != nil {
		t.Errorf("EnsureDefaultRoles() error on second call = %v", err)
	}
	
	// Verify roles are the same
	if everyone2.Name != "everyone" {
		t.Error("EnsureDefaultRoles() should return existing everyone role")
	}
	
	if admin2.Name != "admin" {
		t.Error("EnsureDefaultRoles() should return existing admin role")
	}
}

// ============================================================================
// EnsureUserDefaultRoles Tests
// ============================================================================

func TestRoleService_EnsureUserDefaultRoles_Success(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "userdefaultrole", false)
	room := createRoleServiceRoom(t, &user.ID)
	createRoleServiceMember(t, user.ID, room.ID, "owner")
	
	service := NewRoleService()
	err := service.EnsureUserDefaultRoles(user.ID, room.ID)
	
	if err != nil {
		t.Errorf("EnsureUserDefaultRoles() error = %v", err)
	}
	
	// Verify user has roles
	var memberRoles []models.MemberRole
	database.DB.Where("user_id = ? AND room_id = ?", user.ID, room.ID).Find(&memberRoles)
	
	if len(memberRoles) < 1 {
		t.Error("EnsureUserDefaultRoles() should assign at least one role")
	}
}

func TestRoleService_EnsureUserDefaultRoles_MemberNotFound(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "nomemberuser", false)
	room := createRoleServiceRoom(t, nil)
	
	service := NewRoleService()
	err := service.EnsureUserDefaultRoles(user.ID, room.ID)
	
	if err == nil {
		t.Error("EnsureUserDefaultRoles() should return error when member not found")
	}
}

// ============================================================================
// GetUserRoleIDs Tests
// ============================================================================

func TestRoleService_GetUserRoleIDs_WithRoles(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "roleiduser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	// Create role
	role := createRoleServiceRole(t, room.ID, "Test Role ID", "testroleid", []string{})
	
	// Assign role to user
	memberRole := models.MemberRole{
		UserID: user.ID,
		RoomID: room.ID,
		RoleID: role.ID,
	}
	database.DB.Create(&memberRole)
	
	service := NewRoleService()
	roleIDs := service.GetUserRoleIDs(user.ID, room.ID)
	
	if !roleIDs[role.ID] {
		t.Errorf("GetUserRoleIDs() should contain role ID %d", role.ID)
	}
}

func TestRoleService_GetUserRoleIDs_NoRoles(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "norolesuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	createRoleServiceMember(t, user.ID, room.ID, "member")
	
	service := NewRoleService()
	roleIDs := service.GetUserRoleIDs(user.ID, room.ID)
	
	if len(roleIDs) != 0 {
		t.Errorf("GetUserRoleIDs() should return empty map for user with no roles")
	}
}

// ============================================================================
// CanUserMentionRole Tests
// ============================================================================

func TestRoleService_CanUserMentionRole_EveryonePermission(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "mentionuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	// Create role that can be mentioned by everyone
	role := &models.Role{
		RoomID:                   room.ID,
		Name:                     "Mentionable Role",
		MentionTag:               "mentionable",
		IsSystem:                 false,
		CanBeMentionedByEveryone: true,
		PermissionsJSON:          "[]",
	}
	database.DB.Create(role)
	
	service := NewRoleService()
	canMention := service.CanUserMentionRole(user.ID, room.ID, role)
	
	if !canMention {
		t.Error("CanUserMentionRole() should return true when CanBeMentionedByEveryone is true")
	}
}

func TestRoleService_CanUserMentionRole_SystemRole(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "systemroleuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	// Create system role
	role := &models.Role{
		RoomID:                   room.ID,
		Name:                     "System Role",
		MentionTag:               "systemrole",
		IsSystem:                 true,
		CanBeMentionedByEveryone: false,
		PermissionsJSON:          "[]",
	}
	database.DB.Create(role)
	
	service := NewRoleService()
	canMention := service.CanUserMentionRole(user.ID, room.ID, role)
	
	if !canMention {
		t.Error("CanUserMentionRole() should return true for system roles")
	}
}

func TestRoleService_CanUserMentionRole_NilRole(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "nilroleuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	service := NewRoleService()
	canMention := service.CanUserMentionRole(user.ID, room.ID, nil)
	
	if canMention {
		t.Error("CanUserMentionRole() should return false for nil role")
	}
}

// ============================================================================
// AddRoleMentionPermission Tests
// ============================================================================

func TestRoleService_AddRoleMentionPermission_Success(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "addpermuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	// Create roles
	sourceRole := createRoleServiceRole(t, room.ID, "Source Role", "source", []string{})
	targetRole := createRoleServiceRole(t, room.ID, "Target Role", "target", []string{})
	
	service := NewRoleService()
	err := service.AddRoleMentionPermission(room.ID, sourceRole.ID, targetRole.ID)
	
	if err != nil {
		t.Errorf("AddRoleMentionPermission() error = %v", err)
	}
	
	// Verify permission created
	var perm models.RoleMentionPermission
	result := database.DB.Where("room_id = ? AND source_role_id = ? AND target_role_id = ?", room.ID, sourceRole.ID, targetRole.ID).First(&perm)
	
	if result.Error != nil {
		t.Error("AddRoleMentionPermission() should create permission")
	}
}

func TestRoleService_AddRoleMentionPermission_AlreadyExists(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "existingpermuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	// Create roles
	sourceRole := createRoleServiceRole(t, room.ID, "Source Role 2", "source2", []string{})
	targetRole := createRoleServiceRole(t, room.ID, "Target Role 2", "target2", []string{})
	
	service := NewRoleService()
	// Add permission first time
	service.AddRoleMentionPermission(room.ID, sourceRole.ID, targetRole.ID)
	
	// Add permission second time - should not error
	err := service.AddRoleMentionPermission(room.ID, sourceRole.ID, targetRole.ID)
	
	if err != nil {
		t.Errorf("AddRoleMentionPermission() should not error when permission already exists")
	}
}

// ============================================================================
// RemoveRoleMentionPermission Tests
// ============================================================================

func TestRoleService_RemoveRoleMentionPermission_Success(t *testing.T) {
	cleanup := setupRoleServiceTestDB(t)
	defer cleanup()
	
	user := createRoleServiceUser(t, "removepermuser", false)
	room := createRoleServiceRoom(t, &user.ID)
	
	// Create roles
	sourceRole := createRoleServiceRole(t, room.ID, "Source Role 3", "source3", []string{})
	targetRole := createRoleServiceRole(t, room.ID, "Target Role 3", "target3", []string{})
	
	service := NewRoleService()
	// Add permission
	service.AddRoleMentionPermission(room.ID, sourceRole.ID, targetRole.ID)
	
	// Remove permission
	err := service.RemoveRoleMentionPermission(room.ID, sourceRole.ID, targetRole.ID)
	
	if err != nil {
		t.Errorf("RemoveRoleMentionPermission() error = %v", err)
	}
	
	// Verify permission deleted
	var perm models.RoleMentionPermission
	result := database.DB.Where("room_id = ? AND source_role_id = ? AND target_role_id = ?", room.ID, sourceRole.ID, targetRole.ID).First(&perm)
	
	if result.Error == nil {
		t.Error("RemoveRoleMentionPermission() should delete permission")
	}
}
