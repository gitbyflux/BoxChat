package services

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/repository"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserAlreadyExists  = errors.New("username already taken")
	ErrUserNotFound       = errors.New("user not found")
)

type AuthService struct {
	userRepo repository.UserRepository
}

// NewAuthService creates a new AuthService with default database repository
func NewAuthService() *AuthService {
	return &AuthService{
		userRepo: repository.NewUserRepository(database.DB),
	}
}

// NewAuthServiceWithRepo creates a new AuthService with custom repository (for testing)
func NewAuthServiceWithRepo(userRepo repository.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

type LoginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	RememberMe  bool   `json:"remember_me"`
}

type RegisterRequest struct {
	Username        string `json:"username" binding:"required"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
	RememberMe      bool   `json:"remember_me"`
}

type LoginResponse struct {
	Success bool        `json:"success"`
	Redirect string     `json:"redirect"`
	User    UserInfo    `json:"user"`
	Session SessionInfo `json:"session"`
}

type UserInfo struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type SessionInfo struct {
	Remember   bool   `json:"remember"`
	CookieName string `json:"cookie_name"`
}

func (s *AuthService) Login(req *LoginRequest) (*models.User, error) {
	// Find user by username (case-insensitive)
	user, err := s.getUserByUsername(req.Username)
	if err != nil {
		return nil, ErrInvalidCredentials
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
	if err := s.updateUserLoginInfo(user.ID, 0, nil, &now); err != nil {
		log.Printf("[AUTH] Failed to update user login info: %v", err)
	}

	return user, nil
}

// getUserByUsername finds user by username (case-insensitive)
func (s *AuthService) getUserByUsername(username string) (*models.User, error) {
	return s.userRepo.GetByUsernameCaseInsensitive(username)
}

// updateUserLoginInfo updates user login information
func (s *AuthService) updateUserLoginInfo(id uint, failedAttempts int, lockoutUntil, lastLoginAt *time.Time) error {
	return s.userRepo.UpdateLoginInfo(id, failedAttempts, lockoutUntil, lastLoginAt)
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
	if err := s.checkUsernameExists(req.Username); err != nil {
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

	if err := s.userRepo.Create(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// checkUsernameExists checks if username already exists
func (s *AuthService) checkUsernameExists(username string) error {
	_, err := s.userRepo.GetByUsernameCaseInsensitive(username)
	if err == nil {
		// User found - username is taken
		return ErrUserAlreadyExists
	}
	// Check if error is "record not found" - this means username is available
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	// Some other database error
	return err
}

func (s *AuthService) GetUserByID(userID uint) (*models.User, error) {
	return s.userRepo.GetByID(userID)
}
