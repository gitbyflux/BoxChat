package services

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/testutil"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Helper Functions for Admin Service Tests
// ============================================================================

func setupAdminServiceTestDB(t *testing.T) func() {
	cfg, cleanup := testutil.SetupTestDB(t)
	_ = cfg // Use config if needed
	return cleanup
}

func createAdminServiceUser(t *testing.T, username string, isSuperuser bool) *models.User {
	t.Helper()
	hashedPassword := hashPasswordService(t, "password123")
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

func createAdminServiceRoom(t *testing.T, ownerID *uint) *models.Room {
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

func createAdminServiceMember(t *testing.T, userID, roomID uint, role string) {
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

func hashPasswordService(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

// ============================================================================
// IsAdmin Tests
// ============================================================================

func TestAdminService_IsAdmin_Superuser(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	service := NewAdminService()

	if !service.IsAdmin(admin.ID) {
		t.Error("IsAdmin() should return true for superuser")
	}
}

func TestAdminService_IsAdmin_RegularUser(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	user := createAdminServiceUser(t, "regularuser", false)
	service := NewAdminService()

	if service.IsAdmin(user.ID) {
		t.Error("IsAdmin() should return false for regular user")
	}
}

func TestAdminService_IsAdmin_NonExistentUser(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	service := NewAdminService()

	if service.IsAdmin(99999) {
		t.Error("IsAdmin() should return false for non-existent user")
	}
}

// ============================================================================
// IsRoomAdmin Tests
// ============================================================================

func TestAdminService_IsRoomAdmin_Superuser(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	room := createAdminServiceRoom(t, nil)
	service := NewAdminService()

	if !service.IsRoomAdmin(admin.ID, room.ID) {
		t.Error("IsRoomAdmin() should return true for superuser")
	}
}

func TestAdminService_IsRoomAdmin_RoomOwner(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	user := createAdminServiceUser(t, "roomowneruser", false)
	room := createAdminServiceRoom(t, &user.ID)
	createAdminServiceMember(t, user.ID, room.ID, "owner")
	service := NewAdminService()

	if !service.IsRoomAdmin(user.ID, room.ID) {
		t.Error("IsRoomAdmin() should return true for room owner")
	}
}

func TestAdminService_IsRoomAdmin_RoomAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	user := createAdminServiceUser(t, "roomadminuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, user.ID, room.ID, "admin")
	service := NewAdminService()

	if !service.IsRoomAdmin(user.ID, room.ID) {
		t.Error("IsRoomAdmin() should return true for room admin")
	}
}

func TestAdminService_IsRoomAdmin_RegularMember(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	user := createAdminServiceUser(t, "memberuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, user.ID, room.ID, "member")
	service := NewAdminService()

	if service.IsRoomAdmin(user.ID, room.ID) {
		t.Error("IsRoomAdmin() should return false for regular member")
	}
}

func TestAdminService_IsRoomAdmin_NotInRoom(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	user := createAdminServiceUser(t, "testuser", false)
	room := createAdminServiceRoom(t, nil)
	service := NewAdminService()

	if service.IsRoomAdmin(user.ID, room.ID) {
		t.Error("IsRoomAdmin() should return false for user not in room")
	}
}

// ============================================================================
// KickUserFromRoom Tests
// ============================================================================

func TestAdminService_KickUserFromRoom_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "member")

	service := NewAdminService()
	err := service.KickUserFromRoom(admin.ID, room.ID, target.ID, "Test kick")

	if err != nil {
		t.Errorf("KickUserFromRoom() error = %v", err)
	}

	// Verify membership deleted
	var membership models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", target.ID, room.ID).First(&membership)
	if result.Error == nil {
		t.Error("KickUserFromRoom() should delete membership")
	}
}

func TestAdminService_KickUserFromRoom_NotAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", false)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)

	service := NewAdminService()
	err := service.KickUserFromRoom(admin.ID, room.ID, target.ID, "Test kick")

	if err != ErrNotAdmin {
		t.Errorf("KickUserFromRoom() error = %v, want %v", err, ErrNotAdmin)
	}
}

// ============================================================================
// MuteUserInRoom Tests
// ============================================================================

func TestAdminService_MuteUserInRoom_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "member")

	service := NewAdminService()
	err := service.MuteUserInRoom(admin.ID, room.ID, target.ID, 30, "Test mute")

	if err != nil {
		t.Errorf("MuteUserInRoom() error = %v", err)
	}

	// Verify member is muted
	var membership models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", target.ID, room.ID).First(&membership)
	if result.Error != nil {
		t.Fatalf("Failed to get membership: %v", result.Error)
	}

	if membership.MutedUntil == nil {
		t.Error("MuteUserInRoom() should set MutedUntil")
	}

	// Check duration (should be ~30 minutes from now)
	expectedMin := time.Now().Add(29 * time.Minute)
	expectedMax := time.Now().Add(31 * time.Minute)
	if membership.MutedUntil.Before(expectedMin) || membership.MutedUntil.After(expectedMax) {
		t.Errorf("MutedUntil = %v, expected between %v and %v", membership.MutedUntil, expectedMin, expectedMax)
	}
}

func TestAdminService_MuteUserInRoom_NotAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", false)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)

	service := NewAdminService()
	err := service.MuteUserInRoom(admin.ID, room.ID, target.ID, 30, "Test mute")

	if err != ErrNotAdmin {
		t.Errorf("MuteUserInRoom() error = %v, want %v", err, ErrNotAdmin)
	}
}

// ============================================================================
// UnmuteUserInRoom Tests
// ============================================================================

func TestAdminService_UnmuteUserInRoom_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	
	// Create muted member
	until := time.Now().Add(30 * time.Minute)
	member := models.Member{
		UserID:     target.ID,
		RoomID:     room.ID,
		MutedUntil: &until,
	}
	database.DB.Create(&member)

	service := NewAdminService()
	err := service.UnmuteUserInRoom(admin.ID, room.ID, target.ID)

	if err != nil {
		t.Errorf("UnmuteUserInRoom() error = %v", err)
	}

	// Verify member is unmuted
	var membership models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", target.ID, room.ID).First(&membership)
	if result.Error != nil {
		t.Fatalf("Failed to get membership: %v", result.Error)
	}

	if membership.MutedUntil != nil {
		t.Error("UnmuteUserInRoom() should clear MutedUntil")
	}
}

func TestAdminService_UnmuteUserInRoom_NotAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", false)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)

	service := NewAdminService()
	err := service.UnmuteUserInRoom(admin.ID, room.ID, target.ID)

	if err != ErrNotAdmin {
		t.Errorf("UnmuteUserInRoom() error = %v, want %v", err, ErrNotAdmin)
	}
}

// ============================================================================
// PromoteUser Tests
// ============================================================================

func TestAdminService_PromoteUser_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "member")

	service := NewAdminService()
	err := service.PromoteUser(admin.ID, room.ID, target.ID, "admin")

	if err != nil {
		t.Errorf("PromoteUser() error = %v", err)
	}

	// Verify role changed
	var membership models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", target.ID, room.ID).First(&membership)
	if result.Error != nil {
		t.Fatalf("Failed to get membership: %v", result.Error)
	}

	if membership.Role != "admin" {
		t.Errorf("PromoteUser() role = %v, want admin", membership.Role)
	}
}

func TestAdminService_PromoteUser_InvalidRole(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "member")

	service := NewAdminService()
	err := service.PromoteUser(admin.ID, room.ID, target.ID, "invalid_role")

	if err == nil || err.Error() != "invalid role" {
		t.Errorf("PromoteUser() error = %v, want 'invalid role'", err)
	}
}

func TestAdminService_PromoteUser_NotInRoom(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)

	service := NewAdminService()
	err := service.PromoteUser(admin.ID, room.ID, target.ID, "admin")

	if err == nil {
		t.Error("PromoteUser() should return error for user not in room")
	}
}

// ============================================================================
// DemoteUser Tests
// ============================================================================

func TestAdminService_DemoteUser_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "admin")

	service := NewAdminService()
	err := service.DemoteUser(admin.ID, room.ID, target.ID)

	if err != nil {
		t.Errorf("DemoteUser() error = %v", err)
	}

	// Verify role changed
	var membership models.Member
	result := database.DB.Where("user_id = ? AND room_id = ?", target.ID, room.ID).First(&membership)
	if result.Error != nil {
		t.Fatalf("Failed to get membership: %v", result.Error)
	}

	if membership.Role != "member" {
		t.Errorf("DemoteUser() role = %v, want member", membership.Role)
	}
}

func TestAdminService_DemoteUser_CannotDemoteOwner(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "owner")

	service := NewAdminService()
	err := service.DemoteUser(admin.ID, room.ID, target.ID)

	if err == nil || err.Error() != "cannot demote owner" {
		t.Errorf("DemoteUser() error = %v, want 'cannot demote owner'", err)
	}
}

// ============================================================================
// ChangeUserPassword Tests
// ============================================================================

func TestAdminService_ChangeUserPassword_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)

	service := NewAdminService()
	err := service.ChangeUserPassword(admin.ID, target.ID, "newpassword123")

	if err != nil {
		t.Errorf("ChangeUserPassword() error = %v", err)
	}

	// Verify password changed by trying to login with new password
	var user models.User
	database.DB.First(&user, target.ID)
	
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("newpassword123"))
	if err != nil {
		t.Error("ChangeUserPassword() password should be changed")
	}
}

func TestAdminService_ChangeUserPassword_NotAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", false)
	target := createAdminServiceUser(t, "targetuser", false)

	service := NewAdminService()
	err := service.ChangeUserPassword(admin.ID, target.ID, "newpassword123")

	if err != ErrNotAdmin {
		t.Errorf("ChangeUserPassword() error = %v, want %v", err, ErrNotAdmin)
	}
}

// ============================================================================
// ChangeOwnPassword Tests
// ============================================================================

func TestAdminService_ChangeOwnPassword_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	user := createAdminServiceUser(t, "testuser", false)

	service := NewAdminService()
	err := service.ChangeOwnPassword(user.ID, "password123", "newpassword123")

	if err != nil {
		t.Errorf("ChangeOwnPassword() error = %v", err)
	}

	// Verify password changed
	var updatedUser models.User
	database.DB.First(&updatedUser, user.ID)
	
	err = bcrypt.CompareHashAndPassword([]byte(updatedUser.Password), []byte("newpassword123"))
	if err != nil {
		t.Error("ChangeOwnPassword() password should be changed")
	}
}

func TestAdminService_ChangeOwnPassword_WrongOldPassword(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	user := createAdminServiceUser(t, "testuser", false)

	service := NewAdminService()
	err := service.ChangeOwnPassword(user.ID, "wrongpassword", "newpassword123")

	if err == nil || err.Error() != "invalid current password" {
		t.Errorf("ChangeOwnPassword() error = %v, want 'invalid current password'", err)
	}
}

func TestAdminService_ChangeOwnPassword_UserNotFound(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	service := NewAdminService()
	err := service.ChangeOwnPassword(99999, "password123", "newpassword123")

	if err == nil {
		t.Error("ChangeOwnPassword() should return error for non-existent user")
	}
}

// ============================================================================
// DeleteUserMessages Tests
// ============================================================================

func TestAdminService_DeleteUserMessages_AllMessages(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	
	// Create channel
	channel := models.Channel{
		Name:   "Test Channel",
		RoomID: room.ID,
	}
	database.DB.Create(&channel)

	// Create message by target user
	message := models.Message{
		UserID:    target.ID,
		ChannelID: channel.ID,
		Content:   "Test message",
	}
	database.DB.Create(&message)

	service := NewAdminService()
	err := service.DeleteUserMessages(admin.ID, target.ID, nil)

	if err != nil {
		t.Errorf("DeleteUserMessages() error = %v", err)
	}

	// Verify message deleted
	var msg models.Message
	result := database.DB.First(&msg, message.ID)
	if result.Error == nil {
		t.Error("DeleteUserMessages() should delete messages")
	}
}

func TestAdminService_DeleteUserMessages_InRoom(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	
	// Create channel
	channel := models.Channel{
		Name:   "Test Channel",
		RoomID: room.ID,
	}
	database.DB.Create(&channel)

	// Create message by target user
	message := models.Message{
		UserID:    target.ID,
		ChannelID: channel.ID,
		Content:   "Test message",
	}
	database.DB.Create(&message)

	service := NewAdminService()
	err := service.DeleteUserMessages(admin.ID, target.ID, &room.ID)

	if err != nil {
		t.Errorf("DeleteUserMessages() error = %v", err)
	}

	// Verify message deleted
	var msg models.Message
	result := database.DB.First(&msg, message.ID)
	if result.Error == nil {
		t.Error("DeleteUserMessages() should delete messages")
	}
}

// ============================================================================
// BanUserInRoom Tests
// ============================================================================

func TestAdminService_BanUserInRoom_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "member")

	service := NewAdminService()
	err := service.BanUserInRoom(admin.ID, room.ID, target.ID, "Test ban", nil, false)

	if err != nil {
		t.Errorf("BanUserInRoom() error = %v", err)
	}

	// Verify ban created
	var ban models.RoomBan
	result := database.DB.Where("room_id = ? AND user_id = ?", room.ID, target.ID).First(&ban)
	if result.Error != nil {
		t.Error("BanUserInRoom() should create ban")
	}
}

func TestAdminService_BanUserInRoom_CannotBanSelf(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, admin.ID, room.ID, "member")

	service := NewAdminService()
	err := service.BanUserInRoom(admin.ID, room.ID, admin.ID, "Test ban", nil, false)

	if err != ErrCannotBanSelf {
		t.Errorf("BanUserInRoom() error = %v, want %v", err, ErrCannotBanSelf)
	}
}

func TestAdminService_BanUserInRoom_NotInRoom(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)

	service := NewAdminService()
	err := service.BanUserInRoom(admin.ID, room.ID, target.ID, "Test ban", nil, false)

	if err != ErrNotInRoom {
		t.Errorf("BanUserInRoom() error = %v, want %v", err, ErrNotInRoom)
	}
}

func TestAdminService_BanUserInRoom_CannotBanAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", true) // Superuser
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "member")

	service := NewAdminService()
	err := service.BanUserInRoom(admin.ID, room.ID, target.ID, "Test ban", nil, false)

	if err != ErrCannotBanAdmin {
		t.Errorf("BanUserInRoom() error = %v, want %v", err, ErrCannotBanAdmin)
	}
}

// ============================================================================
// GlobalBanUser Tests
// ============================================================================

func TestAdminService_GlobalBanUser_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	room := createAdminServiceRoom(t, nil)
	createAdminServiceMember(t, target.ID, room.ID, "member")

	service := NewAdminService()
	err := service.GlobalBanUser(admin.ID, target.ID, "Test global ban", false, nil, false)

	if err != nil {
		t.Errorf("GlobalBanUser() error = %v", err)
	}

	// Verify user banned
	var user models.User
	database.DB.First(&user, target.ID)
	if !user.IsBanned {
		t.Error("GlobalBanUser() should ban user")
	}
}

func TestAdminService_GlobalBanUser_CannotBanSelf(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)

	service := NewAdminService()
	err := service.GlobalBanUser(admin.ID, admin.ID, "Test ban", false, nil, false)

	if err != ErrCannotBanSelf {
		t.Errorf("GlobalBanUser() error = %v, want %v", err, ErrCannotBanSelf)
	}
}

func TestAdminService_GlobalBanUser_CannotBanAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", true) // Superuser

	service := NewAdminService()
	err := service.GlobalBanUser(admin.ID, target.ID, "Test ban", false, nil, false)

	if err != ErrCannotBanAdmin {
		t.Errorf("GlobalBanUser() error = %v, want %v", err, ErrCannotBanAdmin)
	}
}

// ============================================================================
// BanUser Tests
// ============================================================================

func TestAdminService_BanUser_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)

	service := NewAdminService()
	err := service.BanUser(admin.ID, target.ID, "Test ban", "192.168.1.1")

	if err != nil {
		t.Errorf("BanUser() error = %v", err)
	}

	// Verify user banned
	var user models.User
	database.DB.First(&user, target.ID)
	if !user.IsBanned {
		t.Error("BanUser() should ban user")
	}
	if user.BanReason != "Test ban" {
		t.Errorf("BanUser() reason = %v, want 'Test ban'", user.BanReason)
	}
}

func TestAdminService_BanUser_CannotBanSelf(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)

	service := NewAdminService()
	err := service.BanUser(admin.ID, admin.ID, "Test ban", "")

	if err != ErrCannotBanSelf {
		t.Errorf("BanUser() error = %v, want %v", err, ErrCannotBanSelf)
	}
}

func TestAdminService_BanUser_CannotBanAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", true)

	service := NewAdminService()
	err := service.BanUser(admin.ID, target.ID, "Test ban", "")

	if err != ErrCannotBanAdmin {
		t.Errorf("BanUser() error = %v, want %v", err, ErrCannotBanAdmin)
	}
}

func TestAdminService_BanUser_NotAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", false)
	target := createAdminServiceUser(t, "targetuser", false)

	service := NewAdminService()
	err := service.BanUser(admin.ID, target.ID, "Test ban", "")

	if err != ErrNotAdmin {
		t.Errorf("BanUser() error = %v, want %v", err, ErrNotAdmin)
	}
}

// ============================================================================
// UnbanUser Tests
// ============================================================================

func TestAdminService_UnbanUser_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)
	
	// First ban the user
	target.IsBanned = true
	target.BanReason = "Test ban"
	database.DB.Save(&target)

	service := NewAdminService()
	err := service.UnbanUser(admin.ID, target.ID)

	if err != nil {
		t.Errorf("UnbanUser() error = %v", err)
	}

	// Verify user unbanned
	var user models.User
	database.DB.First(&user, target.ID)
	if user.IsBanned {
		t.Error("UnbanUser() should unban user")
	}
}

func TestAdminService_UnbanUser_NotBanned(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	target := createAdminServiceUser(t, "targetuser", false)

	service := NewAdminService()
	err := service.UnbanUser(admin.ID, target.ID)

	if err != ErrUserNotBanned {
		t.Errorf("UnbanUser() error = %v, want %v", err, ErrUserNotBanned)
	}
}

// ============================================================================
// GetBannedIPs Tests
// ============================================================================

func TestAdminService_GetBannedIPs_Success(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", true)
	
	// Create banned user with IPs
	bannedUser := createAdminServiceUser(t, "banneduser", false)
	bannedUser.IsBanned = true
	bannedUser.BannedIPs = "192.168.1.1,192.168.1.2"
	database.DB.Save(&bannedUser)

	service := NewAdminService()
	users, err := service.GetBannedIPs(admin.ID)

	if err != nil {
		t.Errorf("GetBannedIPs() error = %v", err)
	}
	if len(users) < 1 {
		t.Error("GetBannedIPs() should return at least 1 user")
	}
}

func TestAdminService_GetBannedIPs_NotAdmin(t *testing.T) {
	cleanup := setupAdminServiceTestDB(t)
	defer cleanup()

	admin := createAdminServiceUser(t, "adminuser", false)

	service := NewAdminService()
	_, err := service.GetBannedIPs(admin.ID)

	if err != ErrNotAdmin {
		t.Errorf("GetBannedIPs() error = %v, want %v", err, ErrNotAdmin)
	}
}
