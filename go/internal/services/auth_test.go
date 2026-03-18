package services

import (
	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// setupTestDB initializes a test database in memory
func setupTestDB(t *testing.T) func() {
	// Reset database state for test
	database.ResetForTesting()
	
	// Create temp directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Set test database URI
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Return cleanup function
	return func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
	}
}

// hashPassword creates a bcrypt hash for testing
func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

// ============================================================================
// Login Tests
// ============================================================================

func TestAuthService_Login_Success(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create test user with hashed password
	hashedPassword := hashPassword(t, "password123")
	user := models.User{
		Username:       "testuser",
		Password:       hashedPassword,
		PresenceStatus: "offline",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	service := NewAuthService()
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
		t.Errorf("Login() username = %v, want %v", result.Username, "testuser")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create test user with hashed password
	hashedPassword := hashPassword(t, "password123")
	user := models.User{
		Username:       "testuser",
		Password:       hashedPassword,
		PresenceStatus: "offline",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	service := NewAuthService()
	req := &LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}

	_, err := service.Login(req)
	if err != ErrInvalidCredentials {
		t.Errorf("Login() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	service := NewAuthService()
	req := &LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	}

	_, err := service.Login(req)
	if err != ErrInvalidCredentials {
		t.Errorf("Login() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestAuthService_Login_BannedUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create banned user with hashed password
	hashedPassword := hashPassword(t, "password123")
	user := models.User{
		Username:       "banneduser",
		Password:       hashedPassword,
		PresenceStatus: "offline",
		IsBanned:       true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	service := NewAuthService()
	req := &LoginRequest{
		Username: "banneduser",
		Password: "password123",
	}

	_, err := service.Login(req)
	if err == nil || err.Error() != "access denied" {
		t.Errorf("Login() error = %v, want 'access denied'", err)
	}
}

func TestAuthService_Login_CaseInsensitive(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create test user with hashed password
	hashedPassword := hashPassword(t, "password123")
	user := models.User{
		Username:       "TestUser",
		Password:       hashedPassword,
		PresenceStatus: "offline",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	service := NewAuthService()
	
	// Test with different case variations
	testCases := []struct {
		name     string
		username string
	}{
		{"lowercase", "testuser"},
		{"uppercase", "TESTUSER"},
		{"mixed case", "TeStUsEr"},
		{"original", "TestUser"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &LoginRequest{
				Username: tc.username,
				Password: "password123",
			}

			result, err := service.Login(req)
			if err != nil {
				t.Errorf("Login(%q) error = %v", tc.username, err)
			}
			if result == nil {
				t.Errorf("Login(%q) returned nil user", tc.username)
			}
		})
	}
}

// ============================================================================
// Register Tests
// ============================================================================

func TestAuthService_Register_Success(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	service := NewAuthService()
	req := &RegisterRequest{
		Username:        "newuser",
		Password:        "password123",
		ConfirmPassword: "password123",
		RememberMe:      false,
	}

	user, err := service.Register(req)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user == nil {
		t.Fatal("Register() returned nil user")
	}
	if user.Username != "newuser" {
		t.Errorf("Register() username = %v, want %v", user.Username, "newuser")
	}
	if user.ID == 0 {
		t.Error("Register() user ID not set")
	}
}

func TestAuthService_Register_UsernameTooShort(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	service := NewAuthService()
	req := &RegisterRequest{
		Username:        "ab",
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	_, err := service.Register(req)
	if err == nil || err.Error() != "username should be 3-30 characters long" {
		t.Errorf("Register() error = %v, want 'username should be 3-30 characters long'", err)
	}
}

func TestAuthService_Register_UsernameTooLong(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	service := NewAuthService()
	req := &RegisterRequest{
		Username:        "this_username_is_way_too_long_and_should_fail",
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	_, err := service.Register(req)
	if err == nil || err.Error() != "username should be 3-30 characters long" {
		t.Errorf("Register() error = %v, want 'username should be 3-30 characters long'", err)
	}
}

func TestAuthService_Register_PasswordTooShort(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	service := NewAuthService()
	req := &RegisterRequest{
		Username:        "testuser",
		Password:        "short",
		ConfirmPassword: "short",
	}

	_, err := service.Register(req)
	if err == nil || err.Error() != "password should be at least 8 characters long" {
		t.Errorf("Register() error = %v, want 'password should be at least 8 characters long'", err)
	}
}

func TestAuthService_Register_PasswordsDoNotMatch(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	service := NewAuthService()
	req := &RegisterRequest{
		Username:        "testuser",
		Password:        "password123",
		ConfirmPassword: "password456",
	}

	_, err := service.Register(req)
	if err == nil || err.Error() != "passwords do not match" {
		t.Errorf("Register() error = %v, want 'passwords do not match'", err)
	}
}

func TestAuthService_Register_UsernameAlreadyExists(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create first user with hashed password
	hashedPassword := hashPassword(t, "password123")
	user := models.User{
		Username:       "existinguser",
		Password:       hashedPassword,
		PresenceStatus: "offline",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	service := NewAuthService()
	req := &RegisterRequest{
		Username:        "existinguser",
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	_, err := service.Register(req)
	if err != ErrUserAlreadyExists {
		t.Errorf("Register() error = %v, want %v", err, ErrUserAlreadyExists)
	}
}

func TestAuthService_Register_CaseInsensitiveUsername(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create first user with hashed password
	hashedPassword := hashPassword(t, "password123")
	user := models.User{
		Username:       "TestUser",
		Password:       hashedPassword,
		PresenceStatus: "offline",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	service := NewAuthService()
	req := &RegisterRequest{
		Username:        "testuser", // Different case
		Password:        "password123",
		ConfirmPassword: "password123",
	}

	_, err := service.Register(req)
	if err != ErrUserAlreadyExists {
		t.Errorf("Register() error = %v, want %v", err, ErrUserAlreadyExists)
	}
}

// ============================================================================
// GetUserByID Tests
// ============================================================================

func TestAuthService_GetUserByID_Success(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create test user
	user := models.User{
		Username:       "testuser",
		Password:       "hashedpassword",
		PresenceStatus: "offline",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	service := NewAuthService()
	result, err := service.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if result == nil {
		t.Fatal("GetUserByID() returned nil")
	}
	if result.Username != "testuser" {
		t.Errorf("GetUserByID() username = %v, want %v", result.Username, "testuser")
	}
}

func TestAuthService_GetUserByID_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	service := NewAuthService()
	_, err := service.GetUserByID(99999)
	if err == nil {
		t.Error("GetUserByID() expected error for non-existent user")
	}
	if err != gorm.ErrRecordNotFound {
		t.Errorf("GetUserByID() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
}

// ============================================================================
// LoginResponse Structure Tests
// ============================================================================

func TestLoginResponseStructure(t *testing.T) {
	resp := LoginResponse{
		Success:  true,
		Redirect: "/home",
		User: UserInfo{
			ID:        1,
			Username:  "testuser",
			AvatarURL: "/uploads/avatars/test.jpg",
		},
		Session: SessionInfo{
			Remember:   true,
			CookieName: "boxchat_session",
		},
	}

	if !resp.Success {
		t.Error("LoginResponse.Success should be true")
	}
	if resp.Redirect != "/home" {
		t.Errorf("LoginResponse.Redirect = %v, want %v", resp.Redirect, "/home")
	}
	if resp.User.ID != 1 {
		t.Errorf("LoginResponse.User.ID = %v, want %v", resp.User.ID, 1)
	}
	if resp.Session.Remember != true {
		t.Error("LoginResponse.Session.Remember should be true")
	}
}
