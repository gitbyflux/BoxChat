package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// MessageRepositoryImpl implements MessageRepository interface
type MessageRepositoryImpl struct {
	db *gorm.DB
}

// NewMessageRepository creates a new MessageRepositoryImpl
func NewMessageRepository(db *gorm.DB) *MessageRepositoryImpl {
	return &MessageRepositoryImpl{db: db}
}

// Create creates a new message in the database
func (r *MessageRepositoryImpl) Create(msg *models.Message) error {
	return r.db.Create(msg).Error
}

// GetByID retrieves a message by ID
func (r *MessageRepositoryImpl) GetByID(id uint) (*models.Message, error) {
	var message models.Message
	err := r.db.Preload("User").Preload("Channel").Preload("Reactions").First(&message, id).Error
	if err != nil {
		return nil, err
	}
	return &message, nil
}

// Update updates an existing message in the database
func (r *MessageRepositoryImpl) Update(msg *models.Message) error {
	return r.db.Save(msg).Error
}

// Delete deletes a message by ID
func (r *MessageRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.Message{}, id).Error
}

// GetLastRead retrieves the last read message for a user in a channel
func (r *MessageRepositoryImpl) GetLastRead(userID, channelID uint) (*models.ReadMessage, error) {
	var read models.ReadMessage
	err := r.db.Where("user_id = ? AND channel_id = ?", userID, channelID).First(&read).Error
	if err != nil {
		return nil, err
	}
	return &read, nil
}

// UpdateRead updates the last read message for a user in a channel
func (r *MessageRepositoryImpl) UpdateRead(read *models.ReadMessage) error {
	return r.db.Save(read).Error
}

// GetByChannel retrieves messages from a channel
func (r *MessageRepositoryImpl) GetByChannel(channelID uint, limit int, offset int64) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.Where("channel_id = ?", channelID).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

// AddReaction adds a reaction to a message
func (r *MessageRepositoryImpl) AddReaction(reaction *models.MessageReaction) error {
	return r.db.Create(reaction).Error
}

// RemoveReaction removes a reaction from a message
func (r *MessageRepositoryImpl) RemoveReaction(reactionID uint) error {
	return r.db.Delete(&models.MessageReaction{}, reactionID).Error
}
