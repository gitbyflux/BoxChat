package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// FriendRepositoryImpl implements FriendRepository interface
type FriendRepositoryImpl struct {
	db *gorm.DB
}

// NewFriendRepository creates a new FriendRepositoryImpl
func NewFriendRepository(db *gorm.DB) *FriendRepositoryImpl {
	return &FriendRepositoryImpl{db: db}
}

// GetFriends retrieves all friends for a user
func (r *FriendRepositoryImpl) GetFriends(userID uint) ([]*models.Friendship, error) {
	var friendships []*models.Friendship
	err := r.db.Where("user_low_id = ? OR user_high_id = ?", userID, userID).
		Preload("UserLow").
		Preload("UserHigh").
		Find(&friendships).Error
	return friendships, err
}

// GetRequests retrieves all friend requests for a user
func (r *FriendRepositoryImpl) GetRequests(userID uint) ([]*models.FriendRequest, error) {
	var requests []*models.FriendRequest
	err := r.db.Where("to_user_id = ? AND status = ?", userID, "pending").
		Preload("FromUser").
		Find(&requests).Error
	return requests, err
}

// CreateRequest creates a new friend request
func (r *FriendRepositoryImpl) CreateRequest(req *models.FriendRequest) error {
	return r.db.Create(req).Error
}

// DeleteRequest deletes a friend request by ID
func (r *FriendRepositoryImpl) DeleteRequest(id uint) error {
	return r.db.Delete(&models.FriendRequest{}, id).Error
}

// CreateFriend creates a new friendship
func (r *FriendRepositoryImpl) CreateFriend(friend *models.Friendship) error {
	return r.db.Create(friend).Error
}

// DeleteFriend deletes a friendship
func (r *FriendRepositoryImpl) DeleteFriend(userID, friendID uint) error {
	return r.db.Where("(user_low_id = ? AND user_high_id = ?) OR (user_low_id = ? AND user_high_id = ?)",
		userID, friendID, friendID, userID).Delete(&models.Friendship{}).Error
}

// GetStatus retrieves the friendship status between two users
func (r *FriendRepositoryImpl) GetStatus(userID, otherID uint) (string, error) {
	// Check if they are friends
	var friendship models.Friendship
	err := r.db.Where("(user_low_id = ? AND user_high_id = ?) OR (user_low_id = ? AND user_high_id = ?)",
		userID, otherID, otherID, userID).First(&friendship).Error

	if err == nil {
		return "friends", nil
	}

	if err != gorm.ErrRecordNotFound {
		return "", err
	}

	// Check if there's a pending request
	var request models.FriendRequest
	err = r.db.Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
		userID, otherID, otherID, userID).First(&request).Error

	if err == nil {
		if request.FromUserID == userID {
			return "pending_outgoing", nil
		}
		return "pending_incoming", nil
	}

	return "none", nil
}

// AcceptRequest accepts a friend request
func (r *FriendRepositoryImpl) AcceptRequest(id uint) error {
	var request models.FriendRequest
	if err := r.db.First(&request, id).Error; err != nil {
		return err
	}

	request.Status = "accepted"
	return r.db.Save(&request).Error
}

// GetByUsers retrieves a friendship between two users
func (r *FriendRepositoryImpl) GetByUsers(userID, otherID uint) (*models.Friendship, error) {
	var friendship models.Friendship
	err := r.db.Where("(user_low_id = ? AND user_high_id = ?) OR (user_low_id = ? AND user_high_id = ?)",
		userID, otherID, otherID, userID).First(&friendship).Error
	if err != nil {
		return nil, err
	}
	return &friendship, nil
}
