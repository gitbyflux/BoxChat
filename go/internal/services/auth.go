package services

import (
	"errors"
	"log"
	"time"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserAlreadyExists  = errors.New("username already taken")
	ErrUserNotFound       = errors.New("user not found")
)

type AuthService struct{}

func NewAuthService() *AuthService {
	return &AuthService{}
}

type LoginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	RememberMe  bool   `json:"remember_me"`
}

type RegisterRequest struct {
	Username       string `json:"username" binding:"required"`
	Password       string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
	RememberMe     bool   `json:"remember_me"`
}

type LoginResponse struct {
	Success   bool      `json:"success"`
	Redirect  string    `json:"redirect"`
	User      UserInfo  `json:"user"`
	Session   SessionInfo `json:"session"`
}

type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type SessionInfo struct {
	Remember   bool   `json:"remember"`
	CookieName string `json:"cookie_name"`
}

func (s *AuthService) Login(req *LoginRequest) (*models.User, error) {
	var user models.User
	
	// Find user by username (case-insensitive)
	if err := database.DB.Where("LOWER(username) = LOWER(?)", req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	
	// Check if user is banned
	if user.IsBanned {
		return nil, errors.New("access denied")
	}
	
	// Check password (try bcrypt first, then fallback to plain for legacy)
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// Fallback: check if it's a legacy plain password (for migration)
		if user.Password != req.Password {
			return nil, ErrInvalidCredentials
		}
	}

	// Update user login info
	now := time.Now()
	user.FailedLoginAttempts = 0
	user.LockoutUntil = nil
	user.LastLoginAt = &now
	if err := database.DB.Save(&user).Error; err != nil {
		log.Printf("[AUTH] Failed to update user login info: %v", err)
	}

	return &user, nil
}

func (s *AuthService) Register(req *RegisterRequest) (*models.User, error) {
	// Validate username
	if len(req.Username) < 3 || len(req.Username) > 30 {
		return nil, errors.New("username should be 3-30 characters long")
	}
	
	// Validate password
	if len(req.Password) < 8 {
		return nil, errors.New("password should be at least 8 characters long")
	}
	
	// Check passwords match
	if req.Password != req.ConfirmPassword {
		return nil, errors.New("passwords do not match")
	}
	
	// Check if username exists
	var existingUser models.User
	if err := database.DB.Where("LOWER(username) = LOWER(?)", req.Username).First(&existingUser).Error; err == nil {
		return nil, ErrUserAlreadyExists
	}
	
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	
	// Create user
	user := models.User{
		Username:            req.Username,
		Password:            string(hashedPassword),
		PresenceStatus:      "offline",
		PrivacySearchable:   true,
		PrivacyListable:     true,
		FailedLoginAttempts: 0,
	}
	
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}
	
	return &user, nil
}

func (s *AuthService) GetUserByID(userID uint) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
