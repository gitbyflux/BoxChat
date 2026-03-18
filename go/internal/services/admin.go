package services

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/repository"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrNotAdmin        = errors.New("not authorized")
	ErrUserNotBanned   = errors.New("user is not banned")
	ErrCannotBanSelf   = errors.New("cannot ban yourself")
	ErrCannotBanAdmin  = errors.New("cannot ban admin")
	ErrNotInRoom       = errors.New("user is not in the room")
	ErrNotEnoughRights = errors.New("not enough rights")
)

type AdminService struct {
	userRepo   repository.UserRepository
	memberRepo repository.MemberRepository
	roomRepo   repository.RoomRepository
	roleRepo   repository.RoleRepository
}

func NewAdminService() *AdminService {
	db := database.DB
	return &AdminService{
		userRepo:   repository.NewUserRepository(db),
		memberRepo: repository.NewMemberRepository(db),
		roomRepo:   repository.NewRoomRepository(db),
		roleRepo:   repository.NewRoleRepository(db),
	}
}

// IsAdmin checks if user is superuser
func (s *AdminService) IsAdmin(userID uint) bool {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false
	}
	return user.IsSuperuser
}

// IsRoomAdmin checks if user has admin rights in room
func (s *AdminService) IsRoomAdmin(userID, roomID uint) bool {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false
	}

	// Superuser can always admin
	if user.IsSuperuser {
		return true
	}

	// Check member role
	member, err := s.memberRepo.GetByRoomAndUser(roomID, userID)
	if err != nil {
		return false
	}

	if member.Role == "owner" || member.Role == "admin" {
		return true
	}

	// Check role permissions
	return s.HasPermissionInRoom(userID, roomID, "ban_members")
}

// HasPermissionInRoom checks if user has specific permission in room
func (s *AdminService) HasPermissionInRoom(userID, roomID uint, permission string) bool {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false
	}

	if user.IsSuperuser {
		return true
	}

	member, err := s.memberRepo.GetByRoomAndUser(roomID, userID)
	if err != nil {
		return false
	}

	if member.Role == "owner" || member.Role == "admin" {
		return true
	}

	// Check role permissions
	memberRoles, err := s.roleRepo.GetMemberRoles(userID, roomID)
	if err != nil {
		return false
	}

	for _, mr := range memberRoles {
		permissions := parseRolePermissions(mr.Role.PermissionsJSON)
		for _, perm := range permissions {
			if perm == permission {
				return true
			}
		}
	}

	return false
}

// BanUser globally bans a user
func (s *AdminService) BanUser(adminID, targetID uint, reason, banIPs string) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	if adminID == targetID {
		return ErrCannotBanSelf
	}

	target, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return err
	}

	if target.IsSuperuser {
		return ErrCannotBanAdmin
	}

	now := time.Now()
	target.IsBanned = true
	target.BanReason = reason
	target.BannedAt = &now
	target.BannedIPs = banIPs

	return s.userRepo.Update(target)
}

// UnbanUser removes global ban from user
func (s *AdminService) UnbanUser(adminID, targetID uint) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	target, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return err
	}

	if !target.IsBanned {
		return ErrUserNotBanned
	}

	target.IsBanned = false
	target.BanReason = ""
	target.BannedAt = nil
	target.BannedIPs = ""

	return s.userRepo.Update(target)
}

// KickUserFromRoom kicks user from a room
func (s *AdminService) KickUserFromRoom(adminID, roomID, targetID uint, reason string) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	return s.memberRepo.DeleteByRoomAndUser(roomID, targetID)
}

// MuteUserInRoom mutes user in a room
func (s *AdminService) MuteUserInRoom(adminID, roomID, targetID uint, durationMinutes int, reason string) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	until := time.Now().Add(time.Duration(durationMinutes) * time.Minute)

	member, err := s.memberRepo.GetByRoomAndUser(roomID, targetID)
	if err != nil {
		return err
	}

	member.MutedUntil = &until
	return s.memberRepo.Update(member)
}

// UnmuteUserInRoom unmutes user in a room
func (s *AdminService) UnmuteUserInRoom(adminID, roomID, targetID uint) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	member, err := s.memberRepo.GetByRoomAndUser(roomID, targetID)
	if err != nil {
		return err
	}

	member.MutedUntil = nil
	return s.memberRepo.Update(member)
}

// PromoteUser promotes user to admin in a room
func (s *AdminService) PromoteUser(adminID, roomID, targetID uint, newRole string) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	if newRole != "admin" && newRole != "owner" && newRole != "member" {
		return errors.New("invalid role")
	}

	member, err := s.memberRepo.GetByRoomAndUser(roomID, targetID)
	if err != nil {
		return err
	}

	member.Role = newRole
	return s.memberRepo.Update(member)
}

// DemoteUser demotes user in a room
func (s *AdminService) DemoteUser(adminID, roomID, targetID uint) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	member, err := s.memberRepo.GetByRoomAndUser(roomID, targetID)
	if err != nil {
		return err
	}

	if member.Role == "owner" {
		return errors.New("cannot demote owner")
	}

	member.Role = "member"
	return s.memberRepo.Update(member)
}

// ChangeUserPassword changes user password (admin action)
func (s *AdminService) ChangeUserPassword(adminID, targetID uint, newPassword string) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	// Validate password length
	if len(newPassword) < 8 {
		return errors.New("password should be at least 8 characters long")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	target, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return err
	}

	target.Password = string(hashedPassword)
	return s.userRepo.Update(target)
}

// DeleteUserMessages deletes all messages by a user
func (s *AdminService) DeleteUserMessages(adminID, targetID uint, roomID *uint) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	db := database.DB

	if roomID != nil {
		// Delete messages in specific room
		// Get all channels in the room first
		var channels []models.Channel
		if err := db.Where("room_id = ?", *roomID).Find(&channels).Error; err != nil {
			return err
		}

		channelIDs := make([]uint, len(channels))
		for i, ch := range channels {
			channelIDs[i] = ch.ID
		}

		if len(channelIDs) > 0 {
			db.Where("user_id = ? AND channel_id IN ?", targetID, channelIDs).
				Delete(&models.Message{})
		}
	} else {
		// Delete all messages
		db.Where("user_id = ?", targetID).Delete(&models.Message{})
	}

	return nil
}

// GetBannedIPs returns list of banned IPs
func (s *AdminService) GetBannedIPs(adminID uint) ([]models.User, error) {
	if !s.IsAdmin(adminID) {
		return nil, ErrNotAdmin
	}

	var users []models.User
	database.DB.Where("is_banned = ? AND banned_ips != ?", true, "").Find(&users)
	return users, nil
}

// ChangeOwnPassword changes current user's password
func (s *AdminService) ChangeOwnPassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	// Check old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return errors.New("invalid current password")
	}

	// Validate new password length
	if len(newPassword) < 8 {
		return errors.New("password should be at least 8 characters long")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return s.userRepo.Update(user)
}

// BanUserInRoom bans user in a specific room
func (s *AdminService) BanUserInRoom(adminID, roomID, targetID uint, reason string, durationMinutes *int, deleteMessages bool) error {
	if adminID == targetID {
		return ErrCannotBanSelf
	}

	// Check if admin has permission
	if !s.IsRoomAdmin(adminID, roomID) {
		return ErrNotEnoughRights
	}

	target, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return err
	}

	if target.IsSuperuser {
		return ErrCannotBanAdmin
	}

	// Use transaction to prevent race conditions
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Lock the target user row to prevent concurrent modifications
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&target, targetID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Check if target is in room (with lock)
	var membership models.Member
	if err := tx.Where("user_id = ? AND room_id = ?", targetID, roomID).First(&membership).Error; err != nil {
		tx.Rollback()
		return ErrNotInRoom
	}

	// Calculate banned_until
	var bannedUntil *time.Time
	if durationMinutes != nil && *durationMinutes > 0 {
		until := time.Now().Add(time.Duration(*durationMinutes) * time.Minute)
		bannedUntil = &until
	}

	// Delete messages if requested
	if deleteMessages {
		var channels []models.Channel
		if err := tx.Where("room_id = ?", roomID).Find(&channels).Error; err == nil {
			channelIDs := make([]uint, len(channels))
			for i, ch := range channels {
				channelIDs[i] = ch.ID
			}
			if len(channelIDs) > 0 {
				tx.Where("user_id = ? AND channel_id IN ?", targetID, channelIDs).Delete(&models.Message{})
			}
		}
	}

	// Use INSERT ... ON CONFLICT UPDATE to atomically create or update ban
	ban := models.RoomBan{
		RoomID:          roomID,
		UserID:          targetID,
		BannedByID:      &adminID,
		Reason:          reason,
		BannedUntil:     bannedUntil,
		MessagesDeleted: deleteMessages,
	}

	// Use Save which will update if exists, create if not
	if err := tx.Save(&ban).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete membership
	if err := tx.Delete(&membership).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// GlobalBanUser bans user globally (superuser only)
func (s *AdminService) GlobalBanUser(adminID, targetID uint, reason string, banIP bool, durationMinutes *int, deleteMessages bool) error {
	if !s.IsAdmin(adminID) {
		return ErrNotAdmin
	}

	if adminID == targetID {
		return ErrCannotBanSelf
	}

	target, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return err
	}

	if target.IsSuperuser {
		return ErrCannotBanAdmin
	}

	now := time.Now()
	target.IsBanned = true
	target.BanReason = reason
	target.BannedAt = &now

	// Ban IP if requested
	if banIP {
		var admin models.User
		if err := database.DB.First(&admin, adminID).Error; err == nil {
			if admin.LastLoginIP != "" {
				target.BannedIPs = admin.LastLoginIP
			}
		}
	}

	database.DB.Save(&target)

	// Calculate banned_until
	var bannedUntil *time.Time
	if durationMinutes != nil && *durationMinutes > 0 {
		until := time.Now().Add(time.Duration(*durationMinutes) * time.Minute)
		bannedUntil = &until
	}

	// Get all memberships
	memberships, err := s.memberRepo.GetByRoom(targetID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	// Create RoomBan records and delete memberships
	for _, m := range memberships {
		ban := models.RoomBan{
			RoomID:          m.RoomID,
			UserID:          targetID,
			BannedByID:      &adminID,
			Reason:          reason,
			BannedUntil:     bannedUntil,
			MessagesDeleted: deleteMessages,
		}
		database.DB.Create(&ban)
		s.memberRepo.Delete(m.ID)
	}

	// Delete messages if requested
	if deleteMessages {
		database.DB.Where("user_id = ?", targetID).Delete(&models.Message{})
	}

	return nil
}
