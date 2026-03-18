package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// ChannelRepositoryImpl implements ChannelRepository interface
type ChannelRepositoryImpl struct {
	db *gorm.DB
}

// NewChannelRepository creates a new ChannelRepositoryImpl
func NewChannelRepository(db *gorm.DB) *ChannelRepositoryImpl {
	return &ChannelRepositoryImpl{db: db}
}

// Create creates a new channel in the database
func (r *ChannelRepositoryImpl) Create(channel *models.Channel) error {
	return r.db.Create(channel).Error
}

// GetByID retrieves a channel by ID
func (r *ChannelRepositoryImpl) GetByID(id uint) (*models.Channel, error) {
	var channel models.Channel
	err := r.db.Preload("Room").First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetByRoom retrieves all channels from a room
func (r *ChannelRepositoryImpl) GetByRoom(roomID uint) ([]*models.Channel, error) {
	var channels []*models.Channel
	err := r.db.Where("room_id = ?", roomID).Find(&channels).Error
	return channels, err
}

// Update updates an existing channel in the database
func (r *ChannelRepositoryImpl) Update(channel *models.Channel) error {
	return r.db.Save(channel).Error
}

// Delete deletes a channel by ID
func (r *ChannelRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Channel{}, id).Error
}

// GetMessages retrieves messages from a channel
func (r *ChannelRepositoryImpl) GetMessages(channelID uint, limit int, offset int64) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.Where("channel_id = ?", channelID).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

// GetByRoomAndName retrieves a channel by room ID and name
func (r *ChannelRepositoryImpl) GetByRoomAndName(roomID uint, name string) (*models.Channel, error) {
	var channel models.Channel
	err := r.db.Where("room_id = ? AND name = ?", roomID, name).First(&channel).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}
