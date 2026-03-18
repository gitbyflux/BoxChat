package repository

import (
	"boxchat/internal/models"
	"time"

	"gorm.io/gorm"
)

// RoleRepositoryImpl implements RoleRepository interface
type RoleRepositoryImpl struct {
	db *gorm.DB
}

// NewRoleRepository creates a new RoleRepositoryImpl
func NewRoleRepository(db *gorm.DB) *RoleRepositoryImpl {
	return &RoleRepositoryImpl{db: db}
}

// Create creates a new role in the database
func (r *RoleRepositoryImpl) Create(role *models.Role) error {
	return r.db.Create(role).Error
}

// GetByID retrieves a role by ID
func (r *RoleRepositoryImpl) GetByID(id uint) (*models.Role, error) {
	var role models.Role
	err := r.db.Preload("Room").First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetByRoom retrieves all roles from a room
func (r *RoleRepositoryImpl) GetByRoom(roomID uint) ([]*models.Role, error) {
	var roles []*models.Role
	err := r.db.Where("room_id = ?", roomID).Find(&roles).Error
	return roles, err
}

// Update updates an existing role in the database
func (r *RoleRepositoryImpl) Update(role *models.Role) error {
	return r.db.Save(role).Error
}

// Delete deletes a role by ID
func (r *RoleRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Role{}, id).Error
}

// GetMemberRoles retrieves all roles for a user in a room
func (r *RoleRepositoryImpl) GetMemberRoles(userID, roomID uint) ([]*models.MemberRole, error) {
	var memberRoles []*models.MemberRole
	err := r.db.Where("user_id = ? AND room_id = ?", userID, roomID).Preload("Role").Find(&memberRoles).Error
	return memberRoles, err
}

// AddMemberRole adds a role to a member
func (r *RoleRepositoryImpl) AddMemberRole(userID, roomID, roleID uint) error {
	now := time.Now()
	memberRole := &models.MemberRole{
		UserID:     userID,
		RoomID:     roomID,
		RoleID:     roleID,
		AssignedAt: now,
	}
	return r.db.Create(memberRole).Error
}

// RemoveMemberRole removes a role from a member
func (r *RoleRepositoryImpl) RemoveMemberRole(userID, roomID, roleID uint) error {
	return r.db.Where("user_id = ? AND room_id = ? AND role_id = ?", userID, roomID, roleID).Delete(&models.MemberRole{}).Error
}

// GetDefaultRole retrieves the default role for a room
func (r *RoleRepositoryImpl) GetDefaultRole(roomID uint) (*models.Role, error) {
	var role models.Role
	err := r.db.Where("room_id = ? AND is_system = ? AND name = ?", roomID, true, "member").First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetAdminRole retrieves the admin role for a room
func (r *RoleRepositoryImpl) GetAdminRole(roomID uint) (*models.Role, error) {
	var role models.Role
	err := r.db.Where("room_id = ? AND is_system = ? AND name = ?", roomID, true, "admin").First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetOwnerRole retrieves the owner role for a room
func (r *RoleRepositoryImpl) GetOwnerRole(roomID uint) (*models.Role, error) {
	var role models.Role
	err := r.db.Where("room_id = ? AND is_system = ? AND name = ?", roomID, true, "owner").First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}
