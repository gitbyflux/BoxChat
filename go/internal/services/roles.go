package services

import (
	"encoding/json"
	"errors"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"gorm.io/gorm"
)

var (
	ErrRoleNotFound        = errors.New("role not found")
	ErrPermissionNotFound  = errors.New("permission not found")
	ErrInvalidPermission   = errors.New("invalid permission key")
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

type RoleService struct{}

func NewRoleService() *RoleService {
	return &RoleService{}
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
	return database.DB.Save(role).Error
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
	var user models.User
	if err := database.DB.First(&user, userID).Error; err == nil && user.IsSuperuser {
		for _, key := range RolePermissionKeys {
			permissions[key] = true
		}
		return permissions
	}

	// Check member role
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).First(&member).Error; err == nil {
		if member.Role == "owner" || member.Role == "admin" {
			for _, key := range RolePermissionKeys {
				permissions[key] = true
			}
			return permissions
		}
	}

	// Get user's roles in room
	var memberRoles []models.MemberRole
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).Find(&memberRoles).Error; err != nil {
		return permissions
	}

	// Collect permissions from all roles
	for _, mr := range memberRoles {
		var role models.Role
		if err := database.DB.First(&role, mr.RoleID).Error; err != nil {
			continue
		}

		rolePerms := s.GetRolePermissions(&role)
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
	var existingEveryone models.Role
	result := database.DB.Where("room_id = ? AND mention_tag = ?", roomID, "everyone").First(&existingEveryone)

	if result.Error == gorm.ErrRecordNotFound {
		everyone = &models.Role{
			RoomID:                   roomID,
			Name:                     "everyone",
			MentionTag:               "everyone",
			IsSystem:                 true,
			CanBeMentionedByEveryone: false,
			PermissionsJSON:          "[]",
		}
		if err := database.DB.Create(everyone).Error; err != nil {
			return nil, nil, err
		}
	} else if result.Error == nil {
		everyone = &existingEveryone
	}

	// Find or create "admin" role
	var existingAdmin models.Role
	result = database.DB.Where("room_id = ? AND mention_tag = ?", roomID, "admin").First(&existingAdmin)

	if result.Error == gorm.ErrRecordNotFound {
		adminPerms, _ := json.Marshal(RolePermissionKeys)
		admin = &models.Role{
			RoomID:                   roomID,
			Name:                     "admin",
			MentionTag:               "admin",
			IsSystem:                 true,
			CanBeMentionedByEveryone: false,
			PermissionsJSON:          string(adminPerms),
		}
		if err := database.DB.Create(admin).Error; err != nil {
			return nil, nil, err
		}
	} else if result.Error == nil {
		admin = &existingAdmin
		// Ensure admin has all permissions
		if admin.PermissionsJSON == "" {
			adminPerms, _ := json.Marshal(RolePermissionKeys)
			admin.PermissionsJSON = string(adminPerms)
			database.DB.Save(admin)
		}
	}

	return everyone, admin, nil
}

// EnsureUserDefaultRoles assigns default roles to user in room
func (s *RoleService) EnsureUserDefaultRoles(userID, roomID uint) error {
	// Check if member exists
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).First(&member).Error; err != nil {
		return err
	}

	// Ensure default roles exist
	everyone, admin, err := s.EnsureDefaultRoles(roomID)
	if err != nil {
		return err
	}

	// Assign "everyone" role if not exists
	var existingEveryoneRole models.MemberRole
	if err := database.DB.Where("user_id = ? AND room_id = ? AND role_id = ?", userID, roomID, everyone.ID).First(&existingEveryoneRole).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			database.DB.Create(&models.MemberRole{
				UserID:  userID,
				RoomID:  roomID,
				RoleID:  everyone.ID,
			})
		}
	}

	// Assign "admin" role if user is owner or admin
	if member.Role == "owner" || member.Role == "admin" {
		var existingAdminRole models.MemberRole
		if err := database.DB.Where("user_id = ? AND room_id = ? AND role_id = ?", userID, roomID, admin.ID).First(&existingAdminRole).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				database.DB.Create(&models.MemberRole{
					UserID:  userID,
					RoomID:  roomID,
					RoleID:  admin.ID,
				})
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
	var permissions []models.RoleMentionPermission
	if err := database.DB.Where("room_id = ? AND target_role_id = ?", roomID, targetRole.ID).Find(&permissions).Error; err != nil {
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

	var memberRoles []models.MemberRole
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).Find(&memberRoles).Error; err != nil {
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
	var existing models.RoleMentionPermission
	if err := database.DB.Where("room_id = ? AND source_role_id = ? AND target_role_id = ?", roomID, sourceRoleID, targetRoleID).First(&existing).Error; err == nil {
		return nil // Already exists
	}

	permission := models.RoleMentionPermission{
		RoomID:       roomID,
		SourceRoleID: sourceRoleID,
		TargetRoleID: targetRoleID,
	}

	return database.DB.Create(&permission).Error
}

// RemoveRoleMentionPermission removes permission for source role to mention target role
func (s *RoleService) RemoveRoleMentionPermission(roomID, sourceRoleID, targetRoleID uint) error {
	return database.DB.Where("room_id = ? AND source_role_id = ? AND target_role_id = ?", roomID, sourceRoleID, targetRoleID).Delete(&models.RoleMentionPermission{}).Error
}
