package services

import (
	"encoding/json"
	"errors"

	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/repository"
)

var (
	ErrRoleNotFound       = errors.New("role not found")
	ErrPermissionNotFound = errors.New("permission not found")
	ErrInvalidPermission  = errors.New("invalid permission key")
)

// RolePermissionKeys defines all available permission keys
var RolePermissionKeys = []string{
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

type RoleService struct {
	roleRepo   repository.RoleRepository
	memberRepo repository.MemberRepository
	userRepo   repository.UserRepository
}

func NewRoleService() *RoleService {
	db := database.DB
	return &RoleService{
		roleRepo:   repository.NewRoleRepository(db),
		memberRepo: repository.NewMemberRepository(db),
		userRepo:   repository.NewUserRepository(db),
	}
}

// GetRolePermissions returns parsed permissions from role
func (s *RoleService) GetRolePermissions(role *models.Role) []string {
	if role.PermissionsJSON == "" {
		return []string{}
	}

	var permissions []string
	if err := json.Unmarshal([]byte(role.PermissionsJSON), &permissions); err != nil {
		return []string{}
	}

	return permissions
}

// SetRolePermissions sets permissions for role
func (s *RoleService) SetRolePermissions(role *models.Role, permissions []string) error {
	// Validate permissions
	validPermissions := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		if s.IsValidPermission(perm) {
			validPermissions = append(validPermissions, perm)
		}
	}

	jsonData, err := json.Marshal(validPermissions)
	if err != nil {
		return err
	}

	role.PermissionsJSON = string(jsonData)
	return s.roleRepo.Update(role)
}

// IsValidPermission checks if permission key is valid
func (s *RoleService) IsValidPermission(permissionKey string) bool {
	for _, key := range RolePermissionKeys {
		if key == permissionKey {
			return true
		}
	}
	return false
}

// GetUserPermissions returns all permissions for user in room
func (s *RoleService) GetUserPermissions(userID, roomID uint) map[string]bool {
	permissions := make(map[string]bool)

	// Check if user is superuser
	user, err := s.userRepo.GetByID(userID)
	if err == nil && user.IsSuperuser {
		for _, key := range RolePermissionKeys {
			permissions[key] = true
		}
		return permissions
	}

	// Check member role
	member, err := s.memberRepo.GetByRoomAndUser(roomID, userID)
	if err == nil {
		if member.Role == "owner" || member.Role == "admin" {
			for _, key := range RolePermissionKeys {
				permissions[key] = true
			}
			return permissions
		}
	}

	// Get user's roles in room
	memberRoles, err := s.roleRepo.GetMemberRoles(userID, roomID)
	if err != nil {
		return permissions
	}

	// Collect permissions from all roles
	for _, mr := range memberRoles {
		rolePerms := s.GetRolePermissions(&mr.Role)
		for _, perm := range rolePerms {
			permissions[perm] = true
		}
	}

	return permissions
}

// UserHasPermission checks if user has specific permission in room
func (s *RoleService) UserHasPermission(userID, roomID uint, permissionKey string) bool {
	if !s.IsValidPermission(permissionKey) {
		return false
	}

	permissions := s.GetUserPermissions(userID, roomID)
	return permissions[permissionKey]
}

// EnsureDefaultRoles creates default roles for room if they don't exist
func (s *RoleService) EnsureDefaultRoles(roomID uint) (*models.Role, *models.Role, error) {
	var everyone *models.Role
	var admin *models.Role

	// Find or create "everyone" role
	existingEveryone, err := s.roleRepo.GetByRoom(roomID)
	if err != nil {
		existingEveryone = []*models.Role{}
	}

	for _, role := range existingEveryone {
		if role.MentionTag == "everyone" {
			everyone = role
			break
		}
	}

	if everyone == nil {
		everyone = &models.Role{
			RoomID:                   roomID,
			Name:                     "everyone",
			MentionTag:               "everyone",
			IsSystem:                 true,
			CanBeMentionedByEveryone: false,
			PermissionsJSON:          "[]",
		}
		if err := s.roleRepo.Create(everyone); err != nil {
			return nil, nil, err
		}
	}

	// Find or create "admin" role
	existingAdmin, err := s.roleRepo.GetByRoom(roomID)
	if err != nil {
		existingAdmin = []*models.Role{}
	}

	for _, role := range existingAdmin {
		if role.MentionTag == "admin" {
			admin = role
			break
		}
	}

	if admin == nil {
		adminPerms, _ := json.Marshal(RolePermissionKeys)
		admin = &models.Role{
			RoomID:                   roomID,
			Name:                     "admin",
			MentionTag:               "admin",
			IsSystem:                 true,
			CanBeMentionedByEveryone: false,
			PermissionsJSON:          string(adminPerms),
		}
		if err := s.roleRepo.Create(admin); err != nil {
			return nil, nil, err
		}
	} else if admin.PermissionsJSON == "" {
		// Ensure admin has all permissions
		adminPerms, _ := json.Marshal(RolePermissionKeys)
		admin.PermissionsJSON = string(adminPerms)
		s.roleRepo.Update(admin)
	}

	return everyone, admin, nil
}

// EnsureUserDefaultRoles assigns default roles to user in room
func (s *RoleService) EnsureUserDefaultRoles(userID, roomID uint) error {
	// Check if member exists
	member, err := s.memberRepo.GetByRoomAndUser(roomID, userID)
	if err != nil {
		return err
	}

	// Ensure default roles exist
	everyone, admin, err := s.EnsureDefaultRoles(roomID)
	if err != nil {
		return err
	}

	// Assign "everyone" role if not exists
	memberRoles, err := s.roleRepo.GetMemberRoles(userID, roomID)
	if err != nil {
		memberRoles = []*models.MemberRole{}
	}

	hasEveryoneRole := false
	hasAdminRole := false

	for _, mr := range memberRoles {
		if mr.RoleID == everyone.ID {
			hasEveryoneRole = true
		}
		if mr.RoleID == admin.ID {
			hasAdminRole = true
		}
	}

	if !hasEveryoneRole {
		if err := s.roleRepo.AddMemberRole(userID, roomID, everyone.ID); err != nil {
			return err
		}
	}

	// Assign "admin" role if user is owner or admin
	if member.Role == "owner" || member.Role == "admin" {
		if !hasAdminRole {
			if err := s.roleRepo.AddMemberRole(userID, roomID, admin.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// CanUserMentionRole checks if user can mention target role
func (s *RoleService) CanUserMentionRole(userID, roomID uint, targetRole *models.Role) bool {
	if targetRole == nil {
		return false
	}

	// If role can be mentioned by everyone, allow
	if targetRole.CanBeMentionedByEveryone {
		return true
	}

	// Check if user has permission via RoleMentionPermission
	db := database.DB
	var permissions []models.RoleMentionPermission
	if err := db.Where("room_id = ? AND target_role_id = ?", roomID, targetRole.ID).Find(&permissions).Error; err != nil {
		return false
	}

	if len(permissions) == 0 {
		// No specific permissions set, check if target role is system role
		if targetRole.IsSystem {
			return true
		}
		return false
	}

	// Get user's roles in this room
	userRoleIDs := s.GetUserRoleIDs(userID, roomID)

	for _, perm := range permissions {
		if userRoleIDs[perm.SourceRoleID] {
			return true
		}
	}

	return false
}

// GetUserRoleIDs returns map of role IDs for user in room
func (s *RoleService) GetUserRoleIDs(userID, roomID uint) map[uint]bool {
	roleIDs := make(map[uint]bool)

	memberRoles, err := s.roleRepo.GetMemberRoles(userID, roomID)
	if err != nil {
		return roleIDs
	}

	for _, mr := range memberRoles {
		roleIDs[mr.RoleID] = true
	}

	return roleIDs
}

// AddRoleMentionPermission adds permission for source role to mention target role
func (s *RoleService) AddRoleMentionPermission(roomID, sourceRoleID, targetRoleID uint) error {
	// Check if permission already exists
	db := database.DB
	var existing models.RoleMentionPermission
	if err := db.Where("room_id = ? AND source_role_id = ? AND target_role_id = ?", roomID, sourceRoleID, targetRoleID).First(&existing).Error; err == nil {
		return nil // Already exists
	}

	permission := models.RoleMentionPermission{
		RoomID:       roomID,
		SourceRoleID: sourceRoleID,
		TargetRoleID: targetRoleID,
	}

	return db.Create(&permission).Error
}

// RemoveRoleMentionPermission removes permission for source role to mention target role
func (s *RoleService) RemoveRoleMentionPermission(roomID, sourceRoleID, targetRoleID uint) error {
	db := database.DB
	return db.Where("room_id = ? AND source_role_id = ? AND target_role_id = ?", roomID, sourceRoleID, targetRoleID).Delete(&models.RoleMentionPermission{}).Error
}
