package services

import (
	"boxchat/internal/mock"
	"boxchat/internal/models"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ============================================================================
// Login Tests - Unit tests with mocks
// ============================================================================

func TestAuthService_Login_Success(t *testing.T) {
	// Create mock user repository
	mockRepo := mock.NewMockUserRepository()
	
	// Create test user with hashed password
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := &models.User{
		BaseModel:      models.BaseModel{ID: 1},
		Username:       "testuser",
		Password:       string(hashedPassword),
		PresenceStatus: "offline",
		IsBanned:       false,
	}

	// Setup mock to return test user
	mockRepo.GetByUsernameCaseInsensitiveFunc = func(username string) (*models.User, error) {
		if username == "testuser" {
			return testUser, nil
		}
		return nil, gorm.ErrRecordNotFound
	}

	// Setup mock to simulate updating login info
	mockRepo.UpdateLoginInfoFunc = func(id uint, failedAttempts int, lockoutUntil, lastLoginAt *time.Time) error {
		return nil
	}

	// Create service with mock repository
	service := NewAuthServiceWithRepo(mockRepo)

	// Test login
	req := &LoginRequest{
		Username:   "testuser",
		Password:   "password123",
		RememberMe: false,
	}

	result, err := service.Login(req)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result == nil {
		t.Fatal("Login() returned nil user")
	}
	if result.Username != "testuser" {
		t.Errorf("Login() username = %v, want testuser", result.Username)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	mockRepo := mock.NewMockUserRepository()
	
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := &models.User{
		BaseModel:      models.BaseModel{ID: 1},
		Username:       "testuser",
		Password:       string(hashedPassword),
		PresenceStatus: "offline",
	}

	mockRepo.GetByUsernameCaseInsensitiveFunc = func(username string) (*models.User, error) {
		return testUser, nil
	}

	service := NewAuthServiceWithRepo(mockRepo)

	req := &LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}

	_, err := service.Login(req)
	if err == nil {
		t.Fatal("Login() should return error for wrong password")
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	mockRepo := mock.NewMockUserRepository()

	mockRepo.GetByUsernameCaseInsensitiveFunc = func(username string) (*models.User, error) {
		return nil, gorm.ErrRecordNotFound
	}

	service := NewAuthServiceWithRepo(mockRepo)

	req := &LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	}

	_, err := service.Login(req)
	if err == nil {
		t.Fatal("Login() should return error for non-existent user")
	}
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_BannedUser(t *testing.T) {
	mockRepo := mock.NewMockUserRepository()
	
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := &models.User{
		BaseModel:      models.BaseModel{ID: 1},
		Username:       "banneduser",
		Password:       string(hashedPassword),
		PresenceStatus: "offline",
		IsBanned:       true,
	}

	mockRepo.GetByUsernameCaseInsensitiveFunc = func(username string) (*models.User, error) {
		return testUser, nil
	}

	service := NewAuthServiceWithRepo(mockRepo)

	req := &LoginRequest{
		Username: "banneduser",
		Password: "password123",
	}

	_, err := service.Login(req)
	if err == nil {
		t.Fatal("Login() should return error for banned user")
	}
	if err.Error() != "access denied" {
		t.Errorf("Login() error = %v, want 'access denied'", err)
	}
}

// ============================================================================
// Register Tests - Unit tests with mocks
// ============================================================================

func TestAuthService_Register_Success(t *testing.T) {
	mockRepo := mock.NewMockUserRepository()

	// Mock to return "user not found" (username is available)
	mockRepo.GetByUsernameCaseInsensitiveFunc = func(username string) (*models.User, error) {
		return nil, gorm.ErrRecordNotFound
	}

	// Mock create to succeed
	mockRepo.CreateFunc = func(user *models.User) error {
		user.ID = 1 // Simulate auto-increment
		return nil
	}

	service := NewAuthServiceWithRepo(mockRepo)

	req := &RegisterRequest{
		Username:        "newuser",
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	result, err := service.Register(req)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result == nil {
		t.Fatal("Register() returned nil user")
	}
	if result.Username != "newuser" {
		t.Errorf("Register() username = %v, want newuser", result.Username)
	}
}

func TestAuthService_Register_ShortUsername(t *testing.T) {
	service := NewAuthService()

	req := &RegisterRequest{
		Username:        "ab",
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	_, err := service.Register(req)
	if err == nil {
		t.Fatal("Register() should return error for short username")
	}
	if err.Error() != "username should be 3-30 characters long" {
		t.Errorf("Register() error = %v, want username length error", err)
	}
}

func TestAuthService_Register_LongUsername(t *testing.T) {
	service := NewAuthService()

	req := &RegisterRequest{
		Username:        "thisusernameiswaytoolongandshouldfail",
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	_, err := service.Register(req)
	if err == nil {
		t.Fatal("Register() should return error for long username")
	}
	if err.Error() != "username should be 3-30 characters long" {
		t.Errorf("Register() error = %v, want username length error", err)
	}
}

func TestAuthService_Register_ShortPassword(t *testing.T) {
	service := NewAuthService()

	req := &RegisterRequest{
		Username:        "testuser",
		Password:        "short",
		ConfirmPassword: "short",
	}

	_, err := service.Register(req)
	if err == nil {
		t.Fatal("Register() should return error for short password")
	}
	if err.Error() != "password should be at least 8 characters long" {
		t.Errorf("Register() error = %v, want password length error", err)
	}
}

func TestAuthService_Register_PasswordsMismatch(t *testing.T) {
	service := NewAuthService()

	req := &RegisterRequest{
		Username:        "testuser",
		Password:        "password123",
		ConfirmPassword: "password456",
	}

	_, err := service.Register(req)
	if err == nil {
		t.Fatal("Register() should return error for mismatched passwords")
	}
	if err.Error() != "passwords do not match" {
		t.Errorf("Register() error = %v, want passwords mismatch error", err)
	}
}

func TestAuthService_Register_UsernameTaken(t *testing.T) {
	mockRepo := mock.NewMockUserRepository()

	// Mock to return existing user
	mockRepo.GetByUsernameCaseInsensitiveFunc = func(username string) (*models.User, error) {
		return &models.User{
			BaseModel: models.BaseModel{ID: 99},
			Username:  username,
		}, nil
	}

	service := NewAuthServiceWithRepo(mockRepo)

	req := &RegisterRequest{
		Username:        "existinguser",
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	_, err := service.Register(req)
	if err == nil {
		t.Fatal("Register() should return error for taken username")
	}
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Errorf("Register() error = %v, want ErrUserAlreadyExists", err)
	}
}

// ============================================================================
// GetUserByID Tests
// ============================================================================

func TestAuthService_GetUserByID_Success(t *testing.T) {
	mockRepo := mock.NewMockUserRepository()

	expectedUser := &models.User{
		BaseModel:      models.BaseModel{ID: 42},
		Username:       "testuser",
		PresenceStatus: "online",
	}

	mockRepo.GetByIDFunc = func(id uint) (*models.User, error) {
		if id == 42 {
			return expectedUser, nil
		}
		return nil, gorm.ErrRecordNotFound
	}

	service := NewAuthServiceWithRepo(mockRepo)

	result, err := service.GetUserByID(42)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if result == nil {
		t.Fatal("GetUserByID() returned nil")
	}
	if result.ID != 42 {
		t.Errorf("GetUserByID() ID = %v, want 42", result.ID)
	}
	if result.Username != "testuser" {
		t.Errorf("GetUserByID() username = %v, want testuser", result.Username)
	}
}

func TestAuthService_GetUserByID_NotFound(t *testing.T) {
	mockRepo := mock.NewMockUserRepository()

	mockRepo.GetByIDFunc = func(id uint) (*models.User, error) {
		return nil, gorm.ErrRecordNotFound
	}

	service := NewAuthServiceWithRepo(mockRepo)

	_, err := service.GetUserByID(999)
	if err == nil {
		t.Fatal("GetUserByID() should return error for non-existent user")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetUserByID() error = %v, want gorm.ErrRecordNotFound", err)
	}
}
