package database

import (
	"boxchat/internal/config"
	"boxchat/internal/models"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB        *gorm.DB
	initOnce  sync.Once
	initError error
	initMu    sync.Mutex
)

// ResetForTesting resets the initialization state for testing purposes.
// This should only be used in tests.
func ResetForTesting() {
	initMu.Lock()
	defer initMu.Unlock()
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
	initOnce = sync.Once{}
	initError = nil
	DB = nil
}

func Init(cfg *config.Config) error {
	initOnce.Do(func() {
		initError = initDatabase(cfg)
	})
	return initError
}

func initDatabase(cfg *config.Config) error {
	// Ensure instance directory exists
	instanceDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(instanceDir, 0755); err != nil {
		return fmt.Errorf("failed to create instance directory: %w", err)
	}
	
	// Open SQLite connection with minimal logging
	var err error
	DB, err = gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn), // Only show warnings and errors
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Enable foreign key constraints for SQLite
	if err := DB.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	fmt.Println("[DATABASE] Foreign key constraints enabled")

	// Run migrations
	if err := AutoMigrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	
	// Create default admin user
	if err := CreateAdminUser(); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	
	fmt.Println("[DATABASE] Connected and migrated successfully")
	return nil
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		// User models
		&models.User{},
		&models.AuthThrottle{},
		&models.UserMusic{},
		&models.Friendship{},
		&models.FriendRequest{},
		
		// Chat models
		&models.Room{},
		&models.Channel{},
		&models.Member{},
		&models.Role{},
		&models.MemberRole{},
		&models.RoleMentionPermission{},
		&models.RoomBan{},
		
		// Content models
		&models.Message{},
		&models.MessageReaction{},
		&models.ReadMessage{},
		&models.StickerPack{},
		&models.Sticker{},
	)
}

func CreateAdminUser() error {
	var admin models.User
	result := DB.Where("username = ?", "admin").First(&admin)

	if result.Error == gorm.ErrRecordNotFound {
		// Get admin password from environment variable or use secure random default
		adminPassword := os.Getenv("ADMIN_PASSWORD")
		if adminPassword == "" {
			// Generate secure random password if not provided
			adminPassword = generateSecurePassword()
			fmt.Printf("[DATABASE] ⚠️  No ADMIN_PASSWORD set. Generated random password: %s\n", adminPassword)
			fmt.Println("[DATABASE] ⚠️  Please set ADMIN_PASSWORD environment variable for security!")
		}

		hashedPassword, err := hashPassword(adminPassword)
		if err != nil {
			return err
		}

		admin = models.User{
			Username:     "admin",
			Password:     hashedPassword,
			IsSuperuser:  true,
			PresenceStatus: "offline",
		}

		if err := DB.Create(&admin).Error; err != nil {
			return err
		}

		fmt.Println("[DATABASE] Admin user created successfully")
	}

	return nil
}

// generateSecurePassword generates a random 16-character password using crypto/rand
func generateSecurePassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, 16)
	randomBytes := make([]byte, 16)
	
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to simple password if random generation fails
		return "ChangeMe123!"
	}
	
	for i := range result {
		result[i] = chars[int(randomBytes[i])%len(chars)]
	}
	return string(result)
}

func hashPassword(password string) (string, error) {
	// Using bcrypt for password hashing
	// Note: For compatibility with Python's scrypt, custom verification is needed
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GetDB() *gorm.DB {
	return DB
}

// InitInMemoryDB initializes an in-memory SQLite database for testing
func InitInMemoryDB() error {
	// Reset any existing connection
	ResetForTesting()
	
	var err error
	DB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to in-memory database: %w", err)
	}

	// Enable foreign key constraints
	if err := DB.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Run migrations
	if err := AutoMigrate(); err != nil {
		return fmt.Errorf("failed to migrate in-memory database: %w", err)
	}

	return nil
}
