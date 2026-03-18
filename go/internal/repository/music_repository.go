package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// MusicRepositoryImpl implements MusicRepository interface
type MusicRepositoryImpl struct {
	db *gorm.DB
}

// NewMusicRepository creates a new MusicRepositoryImpl
func NewMusicRepository(db *gorm.DB) *MusicRepositoryImpl {
	return &MusicRepositoryImpl{db: db}
}

// Create creates a new music track in the database
func (r *MusicRepositoryImpl) Create(track *models.UserMusic) error {
	return r.db.Create(track).Error
}

// GetByUser retrieves all music tracks owned by a user
func (r *MusicRepositoryImpl) GetByUser(userID uint) ([]*models.UserMusic, error) {
	var tracks []*models.UserMusic
	err := r.db.Where("user_id = ?", userID).Find(&tracks).Error
	return tracks, err
}

// Delete deletes a music track by ID
func (r *MusicRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.UserMusic{}, id).Error
}

// GetByID retrieves a music track by ID
func (r *MusicRepositoryImpl) GetByID(id uint) (*models.UserMusic, error) {
	var track models.UserMusic
	err := r.db.First(&track, id).Error
	if err != nil {
		return nil, err
	}
	return &track, nil
}
