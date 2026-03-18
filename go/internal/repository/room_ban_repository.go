package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// RoomBanRepositoryImpl implements RoomBanRepository interface
type RoomBanRepositoryImpl struct {
	db *gorm.DB
}

// NewRoomBanRepository creates a new RoomBanRepositoryImpl
func NewRoomBanRepository(db *gorm.DB) *RoomBanRepositoryImpl {
	return &RoomBanRepositoryImpl{db: db}
}

// Create creates a new room ban in the database
func (r *RoomBanRepositoryImpl) Create(ban *models.RoomBan) error {
	return r.db.Create(ban).Error
}

// GetByID retrieves a room ban by ID
func (r *RoomBanRepositoryImpl) GetByID(id uint) (*models.RoomBan, error) {
	var ban models.RoomBan
	err := r.db.Preload("Room").Preload("User").Preload("BannedBy").First(&ban, id).Error
	if err != nil {
		return nil, err
	}
	return &ban, nil
}

// GetByRoom retrieves all bans from a room
func (r *RoomBanRepositoryImpl) GetByRoom(roomID uint) ([]*models.RoomBan, error) {
	var bans []*models.RoomBan
	err := r.db.Where("room_id = ?", roomID).Find(&bans).Error
	return bans, err
}

// GetByUser retrieves a ban for a user in a room
func (r *RoomBanRepositoryImpl) GetByUser(roomID, userID uint) (*models.RoomBan, error) {
	var ban models.RoomBan
	err := r.db.Where("room_id = ? AND user_id = ?", roomID, userID).First(&ban).Error
	if err != nil {
		return nil, err
	}
	return &ban, nil
}

// Delete deletes a room ban by ID
func (r *RoomBanRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.RoomBan{}, id).Error
}

// DeleteByRoomAndUser deletes a room ban by room ID and user ID
func (r *RoomBanRepositoryImpl) DeleteByRoomAndUser(roomID, userID uint) error {
	return r.db.Where("room_id = ? AND user_id = ?", roomID, userID).Delete(&models.RoomBan{}).Error
}
