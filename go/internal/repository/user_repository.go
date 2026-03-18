package repository

import (
	"boxchat/internal/models"
	"time"

	"gorm.io/gorm"
)

// UserRepositoryImpl implements UserRepository interface
type UserRepositoryImpl struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepositoryImpl
func NewUserRepository(db *gorm.DB) *UserRepositoryImpl {
	return &UserRepositoryImpl{db: db}
}

// Create creates a new user in the database
func (r *UserRepositoryImpl) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// GetByID retrieves a user by ID
func (r *UserRepositoryImpl) GetByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername retrieves a user by username (case-sensitive)
func (r *UserRepositoryImpl) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsernameCaseInsensitive retrieves a user by username (case-insensitive)
func (r *UserRepositoryImpl) GetByUsernameCaseInsensitive(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("LOWER(username) = LOWER(?)", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates an existing user in the database
func (r *UserRepositoryImpl) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// Delete deletes a user by ID
func (r *UserRepositoryImpl) Delete(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

// GetAll retrieves all users from the database
func (r *UserRepositoryImpl) GetAll() ([]*models.User, error) {
	var users []*models.User
	err := r.db.Find(&users).Error
	return users, err
}

// Search searches for users by username
func (r *UserRepositoryImpl) Search(query string, limit int) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Where("username LIKE ?", "%"+query+"%").Limit(limit).Find(&users).Error
	return users, err
}

// UpdateLoginInfo updates user login information
func (r *UserRepositoryImpl) UpdateLoginInfo(id uint, failedAttempts int, lockoutUntil, lastLoginAt *time.Time) error {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return err
	}

	user.FailedLoginAttempts = failedAttempts
	user.LockoutUntil = lockoutUntil
	user.LastLoginAt = lastLoginAt

	return r.db.Save(&user).Error
}
