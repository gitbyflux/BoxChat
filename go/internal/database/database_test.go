package database

import (
	"boxchat/internal/config"
	"boxchat/internal/models"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================================================================
// Init Tests
// ============================================================================

func TestInit_Success(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("ADMIN_PASSWORD", "TestAdminPass123!")
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("ADMIN_PASSWORD")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify database is initialized
	if DB == nil {
		t.Fatal("DB should be initialized")
	}

	// Verify admin user was created
	var admin models.User
	if err := DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Errorf("Admin user should be created: %v", err)
	}
	if !admin.IsSuperuser {
		t.Error("Admin user should be superuser")
	}
}

func TestInit_CreatesInstanceDirectory(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "instance", "subdir", "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify directory was created
	instanceDir := filepath.Dir(dbPath)
	if _, err := os.Stat(instanceDir); os.IsNotExist(err) {
		t.Error("Init() should create instance directory")
	}
}

func TestInit_EnablesForeignKeys(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify foreign keys are enabled
	var result int
	if err := DB.Raw("PRAGMA foreign_keys").Scan(&result).Error; err != nil {
		t.Fatalf("Failed to check foreign keys: %v", err)
	}
	if result != 1 {
		t.Error("Foreign keys should be enabled")
	}
}

func TestInit_AdminUserWithCustomPassword(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("ADMIN_PASSWORD", "MyCustomPassword123!")
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("ADMIN_PASSWORD")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify admin user exists
	var admin models.User
	if err := DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("Admin user should be created: %v", err)
	}

	// Verify password was hashed (starts with $2a$ for bcrypt)
	if len(admin.Password) < 10 || admin.Password[:4] != "$2a$" {
		t.Error("Admin password should be hashed with bcrypt")
	}
}

func TestInit_AdminUserAlreadyExists(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// First init
	err = Init(cfg)
	if err != nil {
		t.Fatalf("First Init() error = %v", err)
	}

	// Second init should not fail (uses sync.Once, returns cached result)
	err = Init(cfg)
	if err != nil {
		t.Fatalf("Second Init() should not error: %v", err)
	}

	// Verify only one admin user exists
	var count int64
	if err := DB.Model(&models.User{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		t.Fatalf("Failed to count admin users: %v", err)
	}
	if count != 1 {
		t.Errorf("Should have exactly 1 admin user, got %d", count)
	}
}

// ============================================================================
// AutoMigrate Tests
// ============================================================================

func TestAutoMigrate_CreatesTables(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify tables exist
	tables := []string{"users", "rooms", "channels", "members", "roles", "messages", "member_roles"}
	for _, table := range tables {
		if !DB.Migrator().HasTable(table) {
			t.Errorf("Table %s should exist after migration", table)
		}
	}
}

func TestAutoMigrate_UserModel(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Create a test user
	user := models.User{
		Username:       "testuser",
		Password:       "hashedpassword",
		PresenceStatus: "online",
		IsSuperuser:    false,
	}
	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user was created with all fields
	var savedUser models.User
	if err := DB.First(&savedUser, user.ID).Error; err != nil {
		t.Fatalf("Failed to fetch user: %v", err)
	}
	if savedUser.Username != "testuser" {
		t.Errorf("Username = %s, want testuser", savedUser.Username)
	}
	if savedUser.PresenceStatus != "online" {
		t.Errorf("PresenceStatus = %s, want online", savedUser.PresenceStatus)
	}
}

// ============================================================================
// CreateAdminUser Tests
// ============================================================================

func TestCreateAdminUser_GeneratesRandomPassword(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Unsetenv("ADMIN_PASSWORD")
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify admin user was created
	var admin models.User
	if err := DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("Admin user should be created: %v", err)
	}

	// Password should be hashed
	if len(admin.Password) < 10 {
		t.Error("Admin password should be hashed")
	}
}

func TestCreateAdminUser_DoesNotOverwrite(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("ADMIN_PASSWORD", "FirstPassword123!")
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("ADMIN_PASSWORD")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// First init with password
	err = Init(cfg)
	if err != nil {
		t.Fatalf("First Init() error = %v", err)
	}

	// Get admin password hash
	var admin1 models.User
	if err := DB.Where("username = ?", "admin").First(&admin1).Error; err != nil {
		t.Fatalf("Failed to fetch admin: %v", err)
	}

	// Change environment password
	os.Setenv("ADMIN_PASSWORD", "SecondPassword456!")

	// Second init should not change the password
	err = Init(cfg)
	if err != nil {
		t.Fatalf("Second Init() error = %v", err)
	}

	// Get admin password hash again
	var admin2 models.User
	if err := DB.Where("username = ?", "admin").First(&admin2).Error; err != nil {
		t.Fatalf("Failed to fetch admin: %v", err)
	}

	// Password should be the same
	if admin1.Password != admin2.Password {
		t.Error("Admin password should not be overwritten on second init")
	}
}

// ============================================================================
// GetDB Tests
// ============================================================================

func TestGetDB_ReturnsDB(t *testing.T) {
	// Reset initialization state for test
	ResetForTesting()
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	db := GetDB()
	if db == nil {
		t.Fatal("GetDB() should return non-nil database")
	}
	if db != DB {
		t.Error("GetDB() should return the global DB")
	}
}

// ============================================================================
// CheckPasswordHash Tests
// ============================================================================

func TestCheckPasswordHash_CorrectPassword(t *testing.T) {
	// Generate a proper bcrypt hash for testing
	password := "testpassword123"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("Failed to generate hash: %v", err)
	}

	if !CheckPasswordHash(password, hash) {
		t.Error("CheckPasswordHash() should return true for correct password")
	}
}

func TestCheckPasswordHash_WrongPassword(t *testing.T) {
	// Generate hash for different password
	hash, err := hashPassword("correctpassword")
	if err != nil {
		t.Fatalf("Failed to generate hash: %v", err)
	}

	if CheckPasswordHash("wrongpassword", hash) {
		t.Error("CheckPasswordHash() should return false for wrong password")
	}
}

func TestCheckPasswordHash_EmptyPassword(t *testing.T) {
	hash, err := hashPassword("somepassword")
	if err != nil {
		t.Fatalf("Failed to generate hash: %v", err)
	}

	if CheckPasswordHash("", hash) {
		t.Error("CheckPasswordHash() should return false for empty password")
	}
}

func TestCheckPasswordHash_InvalidHash(t *testing.T) {
	if CheckPasswordHash("testpassword", "not_a_valid_hash") {
		t.Error("CheckPasswordHash() should return false for invalid hash")
	}
}

// ============================================================================
// InitInMemoryDB Tests
// ============================================================================

func TestInitInMemoryDB_Success(t *testing.T) {
	// Clean up any existing DB
	if DB != nil {
		sqlDB, _ := DB.DB()
		sqlDB.Close()
	}

	err := InitInMemoryDB()
	if err != nil {
		t.Fatalf("InitInMemoryDB() error = %v", err)
	}

	// Verify database is initialized
	if DB == nil {
		t.Fatal("DB should be initialized")
	}

	// Verify tables exist
	if !DB.Migrator().HasTable("users") {
		t.Error("InitInMemoryDB() should create users table")
	}

	// Clean up
	sqlDB, _ := DB.DB()
	sqlDB.Close()
}

// ============================================================================
// GetDB Tests
// ============================================================================

func TestGetDB_AfterInit(t *testing.T) {
	// First initialize DB
	err := InitInMemoryDB()
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer func() {
		sqlDB, _ := DB.DB()
		sqlDB.Close()
	}()

	db := GetDB()
	if db == nil {
		t.Fatal("GetDB() should return non-nil database")
	}
	if db != DB {
		t.Error("GetDB() should return the global DB")
	}
}

// ============================================================================
// AutoMigrate Tests
// ============================================================================

func TestAutoMigrate_Standalone(t *testing.T) {
	// Initialize fresh in-memory DB without auto-migrate
	var err error
	DB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		sqlDB, _ := DB.DB()
		sqlDB.Close()
	}()

	// Run AutoMigrate
	err = AutoMigrate()
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	// Verify tables were created
	tables := []string{"users", "rooms", "channels", "members", "roles", "messages"}
	for _, table := range tables {
		if !DB.Migrator().HasTable(table) {
			t.Errorf("AutoMigrate() should create table %s", table)
		}
	}
}

// ============================================================================
// generateSecurePassword Tests
// ============================================================================

func TestGenerateSecurePassword_Length(t *testing.T) {
	password := generateSecurePassword()

	if len(password) != 16 {
		t.Errorf("generateSecurePassword() length = %d, want 16", len(password))
	}
}

func TestGenerateSecurePassword_Uniqueness(t *testing.T) {
	passwords := make(map[string]bool)
	for i := 0; i < 100; i++ {
		password := generateSecurePassword()
		if passwords[password] {
			t.Error("generateSecurePassword() generated duplicate password")
		}
		passwords[password] = true
	}
}

func TestGenerateSecurePassword_Charset(t *testing.T) {
	password := generateSecurePassword()

	// Verify password contains only allowed characters
	allowedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	for _, c := range password {
		found := false
		for _, allowed := range allowedChars {
			if c == allowed {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("generateSecurePassword() contains invalid character: %c", c)
		}
	}
}

// ============================================================================
// hashPassword Tests
// ============================================================================

func TestHashPassword_BcryptFormat(t *testing.T) {
	hash, err := hashPassword("testpassword")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	// Bcrypt hashes start with $2a$
	if len(hash) < 10 || hash[:4] != "$2a$" {
		t.Errorf("hashPassword() should return bcrypt hash, got: %s", hash)
	}
}

func TestHashPassword_UniqueHashes(t *testing.T) {
	password := "samepassword"
	hash1, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	hash2, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	// Bcrypt includes salt, so hashes should be different
	if hash1 == hash2 {
		t.Error("hashPassword() should generate unique hashes for same password")
	}
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	hash, err := hashPassword("")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	if hash == "" {
		t.Error("hashPassword() should return hash for empty password")
	}
}
