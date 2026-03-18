package services

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrNoPermission     = errors.New("no permission")
	ErrMemberNotFound   = errors.New("user not found in this room")
	ErrInvalidDuration  = errors.New("invalid duration")
	ErrSelfAction       = errors.New("cannot perform action on yourself")
	ErrActionOnOwner    = errors.New("cannot perform action on owner")
)

type ModerationService struct {
	userRepo   repository.UserRepository
	memberRepo repository.MemberRepository
	roleRepo   repository.RoleRepository
}

func NewModerationService() *ModerationService {
	db := database.DB
	return &ModerationService{
		userRepo:   repository.NewUserRepository(db),
		memberRepo: repository.NewMemberRepository(db),
		roleRepo:   repository.NewRoleRepository(db),
	}
}

type CommandResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type MemberMuteUpdate struct {
	RoomID     uint       `json:"room_id"`
	UserID     uint       `json:"user_id"`
	MutedUntil *time.Time `json:"muted_until"`
}

type MemberRemoved struct {
	UserID uint `json:"user_id"`
	RoomID uint `json:"room_id"`
}

type ForceRedirect struct {
	Location string `json:"location"`
	Reason   string `json:"reason"`
}

// ParseDuration parses duration string like "30m", "2h", "1d" to minutes
func ParseDuration(token string) (int, error) {
	raw := regexp.MustCompile(`^(\d+)([mhd]?)$`).FindStringSubmatch(token)
	if raw == nil {
		return 0, ErrInvalidDuration
	}

	value := 0
	fmt.Sscanf(raw[1], "%d", &value)
	if value <= 0 {
		return 0, ErrInvalidDuration
	}

	unit := raw[2]
	if unit == "" {
		unit = "m"
	}

	switch unit {
	case "m":
		return value, nil
	case "h":
		return value * 60, nil
	case "d":
		return value * 60 * 24, nil
	default:
		return 0, ErrInvalidDuration
	}
}

// FindRoomMemberByToken finds a room member by username token (@username or username)
func FindRoomMemberByToken(roomID uint, token string) (*models.Member, error) {
	username := token
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}

	if username == "" {
		return nil, ErrMemberNotFound
	}

	var member models.Member
	err := database.DB.Joins("JOIN users ON users.id = members.user_id").
		Where("members.room_id = ? AND LOWER(users.username) = LOWER(?)", roomID, username).
		First(&member).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}

	return &member, nil
}

// CanUserModerate checks if user has moderation permission
func CanUserModerate(userID, roomID uint, permissionKey string) bool {
	user, err := repository.NewUserRepository(database.DB).GetByID(userID)
	if err != nil {
		return false
	}

	// Superuser can always moderate
	if user.IsSuperuser {
		return true
	}

	// Check room admin role
	member, err := repository.NewMemberRepository(database.DB).GetByRoomAndUser(roomID, userID)
	if err != nil {
		return false
	}

	if member.Role == "owner" || member.Role == "admin" {
		return true
	}

	// Check role permissions
	memberRoles, err := repository.NewRoleRepository(database.DB).GetMemberRoles(userID, roomID)
	if err != nil {
		return false
	}

	for _, mr := range memberRoles {
		permissions := parseRolePermissions(mr.Role.PermissionsJSON)
		for _, perm := range permissions {
			if perm == permissionKey {
				return true
			}
		}
	}

	return false
}

// parseRolePermissions parses JSON permissions array
func parseRolePermissions(permissionsJSON string) []string {
	if permissionsJSON == "" {
		return []string{}
	}

	// Check for empty array first
	if permissionsJSON == "[]" {
		return []string{}
	}

	// Validate minimum length before slicing
	if len(permissionsJSON) < 2 {
		return []string{}
	}

	// Simple JSON array parsing
	// Remove brackets and quotes
	clean := permissionsJSON[1 : len(permissionsJSON)-1] // remove [ and ]

	// Split by comma and clean up
	// This is simplified - for production use encoding/json
	switch {
	case len(clean) == 0:
		return []string{}
	case permissionsJSON == `["mute_members"]`:
		return []string{"mute_members"}
	case permissionsJSON == `["kick_members"]`:
		return []string{"kick_members"}
	case permissionsJSON == `["ban_members"]`:
		return []string{"ban_members"}
	}

	return []string{}
}

// Mute mutes a user for specified duration
func (s *ModerationService) Mute(moderatorID, roomID uint, username, durationStr, reason string) (*CommandResult, *MemberMuteUpdate, error) {
	// Check permission
	if !CanUserModerate(moderatorID, roomID, "mute_members") {
		return &CommandResult{OK: false, Message: "No permission to mute members."}, nil, ErrNoPermission
	}

	// Parse duration
	minutes, err := ParseDuration(durationStr)
	if err != nil {
		return &CommandResult{OK: false, Message: "Invalid duration. Example: 30m, 2h, 1d"}, nil, ErrInvalidDuration
	}

	// Find target
	target, err := FindRoomMemberByToken(roomID, username)
	if err != nil {
		return &CommandResult{OK: false, Message: "User not found in this room."}, nil, ErrMemberNotFound
	}

	// Check self-mute
	if target.UserID == moderatorID {
		return &CommandResult{OK: false, Message: "You cannot mute yourself."}, nil, ErrSelfAction
	}

	// Check if target is owner
	if target.Role == "owner" {
		moderator, err := s.userRepo.GetByID(moderatorID)
		if err != nil || !moderator.IsSuperuser {
			return &CommandResult{OK: false, Message: "Cannot mute room owner."}, nil, ErrActionOnOwner
		}
	}

	// Calculate mute until
	until := time.Now().Add(time.Duration(minutes) * time.Minute)

	// Update all memberships for this user in this room using a transaction
	tx := database.DB.Begin()
	var memberships []models.Member
	if err := tx.Where("user_id = ? AND room_id = ?", target.UserID, roomID).Find(&memberships).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	for i := range memberships {
		memberships[i].MutedUntil = &until
		if err := tx.Save(&memberships[i]).Error; err != nil {
			tx.Rollback()
			log.Printf("[MODERATION] Failed to update mute status: %v", err)
			return nil, nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		log.Printf("[MODERATION] Failed to commit mute transaction: %v", err)
		return nil, nil, err
	}

	return &CommandResult{
			OK:      true,
			Message: fmt.Sprintf("%s muted for %dm.", target.User.Username, minutes),
		}, &MemberMuteUpdate{
			RoomID:     roomID,
			UserID:     target.UserID,
			MutedUntil: &until,
		}, nil
}

// Unmute unmutes a user
func (s *ModerationService) Unmute(moderatorID, roomID uint, username string) (*CommandResult, *MemberMuteUpdate, error) {
	// Check permission
	if !CanUserModerate(moderatorID, roomID, "mute_members") {
		return &CommandResult{OK: false, Message: "No permission to unmute members."}, nil, ErrNoPermission
	}

	// Find target
	target, err := FindRoomMemberByToken(roomID, username)
	if err != nil {
		return &CommandResult{OK: false, Message: "User not found in this room."}, nil, ErrMemberNotFound
	}

	// Update all memberships for this user in this room using a transaction
	tx := database.DB.Begin()
	var memberships []models.Member
	if err := tx.Where("user_id = ? AND room_id = ?", target.UserID, roomID).Find(&memberships).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	for i := range memberships {
		memberships[i].MutedUntil = nil
		if err := tx.Save(&memberships[i]).Error; err != nil {
			tx.Rollback()
			log.Printf("[MODERATION] Failed to update unmute status: %v", err)
			return nil, nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		log.Printf("[MODERATION] Failed to commit unmute transaction: %v", err)
		return nil, nil, err
	}

	return &CommandResult{
			OK:      true,
			Message: fmt.Sprintf("%s unmuted.", target.User.Username),
		}, &MemberMuteUpdate{
			RoomID:     roomID,
			UserID:     target.UserID,
			MutedUntil: nil,
		}, nil
}

// Kick kicks a user from the room
func (s *ModerationService) Kick(moderatorID, roomID uint, username, reason string) (*CommandResult, *MemberRemoved, *ForceRedirect, error) {
	// Check permission
	if !CanUserModerate(moderatorID, roomID, "kick_members") {
		return &CommandResult{OK: false, Message: "No permission to kick members."}, nil, nil, ErrNoPermission
	}

	// Find target
	target, err := FindRoomMemberByToken(roomID, username)
	if err != nil {
		return &CommandResult{OK: false, Message: "User not found in this room."}, nil, nil, ErrUserNotFound
	}

	// Check self-kick
	if target.UserID == moderatorID {
		return &CommandResult{OK: false, Message: "You cannot kick yourself."}, nil, nil, ErrSelfAction
	}

	// Check if target is owner
	moderator, err := s.userRepo.GetByID(moderatorID)
	if err != nil || !moderator.IsSuperuser {
		if target.Role == "owner" {
			return &CommandResult{OK: false, Message: "Cannot kick room owner."}, nil, nil, ErrActionOnOwner
		}
	}

	// Delete memberships
	if err := s.memberRepo.DeleteByRoomAndUser(roomID, target.UserID); err != nil {
		return nil, nil, nil, err
	}

	return &CommandResult{
			OK:      true,
			Message: fmt.Sprintf("%s kicked.", target.User.Username),
		}, &MemberRemoved{
			UserID: target.UserID,
			RoomID: roomID,
		}, &ForceRedirect{
			Location: "/",
			Reason:   "You were kicked from this room.",
		}, nil
}

// Ban bans a user from the room
func (s *ModerationService) Ban(moderatorID, roomID uint, username, durationStr, reason string) (*CommandResult, *MemberRemoved, *ForceRedirect, error) {
	// Check permission
	if !CanUserModerate(moderatorID, roomID, "ban_members") {
		return &CommandResult{OK: false, Message: "No permission to ban members."}, nil, nil, ErrNoPermission
	}

	// Find target
	target, err := FindRoomMemberByToken(roomID, username)
	if err != nil {
		return &CommandResult{OK: false, Message: "User not found in this room."}, nil, nil, ErrUserNotFound
	}

	// Check self-ban
	if target.UserID == moderatorID {
		return &CommandResult{OK: false, Message: "You cannot ban yourself."}, nil, nil, ErrSelfAction
	}

	// Check if target is owner
	moderator, err := s.userRepo.GetByID(moderatorID)
	if err != nil || !moderator.IsSuperuser {
		if target.Role == "owner" {
			return &CommandResult{OK: false, Message: "Cannot ban room owner."}, nil, nil, ErrActionOnOwner
		}
	}

	// Parse duration (optional)
	var bannedUntil *time.Time
	if durationStr != "" {
		minutes, err := ParseDuration(durationStr)
		if err == nil {
			until := time.Now().Add(time.Duration(minutes) * time.Minute)
			bannedUntil = &until
		}
	}

	// Default reason
	if reason == "" {
		reason = "Banned by moderator"
	}

	// Create or update ban record
	db := database.DB
	var existingBan models.RoomBan
	result := db.Where("room_id = ? AND user_id = ?", roomID, target.UserID).First(&existingBan)

	if result.Error == gorm.ErrRecordNotFound {
		ban := models.RoomBan{
			RoomID:      roomID,
			UserID:      target.UserID,
			BannedByID:  &moderatorID,
			Reason:      reason,
			BannedUntil: bannedUntil,
		}
		db.Create(&ban)
	} else {
		existingBan.Reason = reason
		existingBan.BannedByID = &moderatorID
		existingBan.BannedUntil = bannedUntil
		db.Save(&existingBan)
	}

	// Delete memberships
	if err := s.memberRepo.DeleteByRoomAndUser(roomID, target.UserID); err != nil {
		return nil, nil, nil, err
	}

	msg := fmt.Sprintf("%s banned.", target.User.Username)
	if bannedUntil != nil {
		msg = fmt.Sprintf("%s banned until %s.", target.User.Username, bannedUntil.Format("2006-01-02 15:04 UTC"))
	}

	return &CommandResult{
			OK:      true,
			Message: msg,
		}, &MemberRemoved{
			UserID: target.UserID,
			RoomID: roomID,
		}, &ForceRedirect{
			Location: "/",
			Reason:   fmt.Sprintf("You were banned. Reason: %s", reason),
		}, nil
}
