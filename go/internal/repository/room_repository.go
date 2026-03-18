package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// RoomRepositoryImpl implements RoomRepository interface
type RoomRepositoryImpl struct {
	db *gorm.DB
}

// NewRoomRepository creates a new RoomRepositoryImpl
func NewRoomRepository(db *gorm.DB) *RoomRepositoryImpl {
	return &RoomRepositoryImpl{db: db}
}

// Create creates a new room in the database
func (r *RoomRepositoryImpl) Create(room *models.Room) error {
	return r.db.Create(room).Error
}

// GetByID retrieves a room by ID
func (r *RoomRepositoryImpl) GetByID(id uint) (*models.Room, error) {
	var room models.Room
	err := r.db.Preload("Owner").Preload("Channels").Preload("Members").Preload("Roles").First(&room, id).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// GetAll retrieves all rooms from the database
func (r *RoomRepositoryImpl) GetAll() ([]*models.Room, error) {
	var rooms []*models.Room
	err := r.db.Find(&rooms).Error
	return rooms, err
}

// Update updates an existing room in the database
func (r *RoomRepositoryImpl) Update(room *models.Room) error {
	return r.db.Save(room).Error
}

// Delete deletes a room by ID
func (r *RoomRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Room{}, id).Error
}

// GetByToken retrieves a room by invite token
func (r *RoomRepositoryImpl) GetByToken(token string) (*models.Room, error) {
	var room models.Room
	err := r.db.Where("invite_token = ?", token).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// GetMember retrieves a member from a room
func (r *RoomRepositoryImpl) GetMember(roomID, userID uint) (*models.Member, error) {
	var member models.Member
	err := r.db.Where("room_id = ? AND user_id = ?", roomID, userID).First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// GetMembers retrieves all members from a room
func (r *RoomRepositoryImpl) GetMembers(roomID uint) ([]*models.Member, error) {
	var members []*models.Member
	err := r.db.Where("room_id = ?", roomID).Find(&members).Error
	return members, err
}

// AddMember adds a member to a room
func (r *RoomRepositoryImpl) AddMember(member *models.Member) error {
	return r.db.Create(member).Error
}

// RemoveMember removes a member from a room
func (r *RoomRepositoryImpl) RemoveMember(roomID, userID uint) error {
	return r.db.Where("room_id = ? AND user_id = ?", roomID, userID).Delete(&models.Member{}).Error
}

// GetBans retrieves all bans from a room
func (r *RoomRepositoryImpl) GetBans(roomID uint) ([]*models.RoomBan, error) {
	var bans []*models.RoomBan
	err := r.db.Where("room_id = ?", roomID).Find(&bans).Error
	return bans, err
}

// AddBan adds a ban to a room
func (r *RoomRepositoryImpl) AddBan(ban *models.RoomBan) error {
	return r.db.Create(ban).Error
}

// RemoveBan removes a ban from a room
func (r *RoomRepositoryImpl) RemoveBan(roomID, userID uint) error {
	return r.db.Where("room_id = ? AND user_id = ?", roomID, userID).Delete(&models.RoomBan{}).Error
}

// GetByOwner retrieves all rooms owned by a user
func (r *RoomRepositoryImpl) GetByOwner(ownerID uint) ([]*models.Room, error) {
	var rooms []*models.Room
	err := r.db.Where("owner_id = ?", ownerID).Find(&rooms).Error
	return rooms, err
}
