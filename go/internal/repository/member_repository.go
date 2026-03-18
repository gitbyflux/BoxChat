package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// MemberRepositoryImpl implements MemberRepository interface
type MemberRepositoryImpl struct {
	db *gorm.DB
}

// NewMemberRepository creates a new MemberRepositoryImpl
func NewMemberRepository(db *gorm.DB) *MemberRepositoryImpl {
	return &MemberRepositoryImpl{db: db}
}

// Create creates a new member in the database
func (r *MemberRepositoryImpl) Create(member *models.Member) error {
	return r.db.Create(member).Error
}

// GetByID retrieves a member by ID
func (r *MemberRepositoryImpl) GetByID(id uint) (*models.Member, error) {
	var member models.Member
	err := r.db.Preload("User").Preload("Room").First(&member, id).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// GetByRoomAndUser retrieves a member by room ID and user ID
func (r *MemberRepositoryImpl) GetByRoomAndUser(roomID, userID uint) (*models.Member, error) {
	var member models.Member
	err := r.db.Where("room_id = ? AND user_id = ?", roomID, userID).
		Preload("User").
		Preload("Room").
		First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// GetByRoom retrieves all members from a room
func (r *MemberRepositoryImpl) GetByRoom(roomID uint) ([]*models.Member, error) {
	var members []*models.Member
	err := r.db.Where("room_id = ?", roomID).
		Preload("User").
		Find(&members).Error
	return members, err
}

// Update updates an existing member in the database
func (r *MemberRepositoryImpl) Update(member *models.Member) error {
	return r.db.Save(member).Error
}

// Delete deletes a member by ID
func (r *MemberRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Member{}, id).Error
}

// DeleteByRoomAndUser deletes a member by room ID and user ID
func (r *MemberRepositoryImpl) DeleteByRoomAndUser(roomID, userID uint) error {
	return r.db.Where("room_id = ? AND user_id = ?", roomID, userID).Delete(&models.Member{}).Error
}
