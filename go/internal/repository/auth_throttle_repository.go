package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// AuthThrottleRepositoryImpl implements AuthThrottleRepository interface
type AuthThrottleRepositoryImpl struct {
	db *gorm.DB
}

// NewAuthThrottleRepository creates a new AuthThrottleRepositoryImpl
func NewAuthThrottleRepository(db *gorm.DB) *AuthThrottleRepositoryImpl {
	return &AuthThrottleRepositoryImpl{db: db}
}

// GetByIP retrieves auth throttle info by IP address
func (r *AuthThrottleRepositoryImpl) GetByIP(ip string) (*models.AuthThrottle, error) {
	var throttle models.AuthThrottle
	err := r.db.Where("ip_address = ?", ip).First(&throttle).Error
	if err != nil {
		return nil, err
	}
	return &throttle, nil
}

// Create creates a new auth throttle entry in the database
func (r *AuthThrottleRepositoryImpl) Create(throttle *models.AuthThrottle) error {
	return r.db.Create(throttle).Error
}

// Update updates an existing auth throttle entry in the database
func (r *AuthThrottleRepositoryImpl) Update(throttle *models.AuthThrottle) error {
	return r.db.Save(throttle).Error
}

// Delete deletes an auth throttle entry by ID
func (r *AuthThrottleRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.AuthThrottle{}, id).Error
}
