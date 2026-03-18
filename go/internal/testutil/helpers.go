// Package testutil provides testing utilities for BoxChat
package testutil

import (
	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDB creates an in-memory SQLite database for testing.
// Returns the config and cleanup function.
//
// This function:
// - Creates a fresh in-memory database
// - Runs all migrations
// - Creates default admin user
// - Returns config that can be used with handlers
//
// Usage:
//   cfg, cleanup := testutil.SetupTestDB(t)
//   defer cleanup()
func SetupTestDB(t *testing.T) (*config.Config, func()) {
	t.Helper()

	// Create a unique in-memory database for this test
	// Using file::memory:?cache=shared with unique name to avoid cross-test contamination
	dbPath := fmt.Sprintf("file:mem_%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())

	// Open direct database connection
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Enable foreign keys
	db.Exec("PRAGMA foreign_keys = ON")

	// Run migrations
	if err := db.AutoMigrate(
		&models.User{},
		&models.AuthThrottle{},
		&models.UserMusic{},
		&models.Friendship{},
		&models.FriendRequest{},
		&models.Room{},
		&models.Channel{},
		&models.Member{},
		&models.Role{},
		&models.MemberRole{},
		&models.RoleMentionPermission{},
		&models.RoomBan{},
		&models.Message{},
		&models.MessageReaction{},
		&models.ReadMessage{},
		&models.StickerPack{},
		&models.Sticker{},
	); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create default admin user
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "TestAdminPass123!"
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	admin := models.User{
		Username:       "admin",
		Password:       string(hashedPassword),
		IsSuperuser:    true,
		PresenceStatus: "offline",
	}
	db.Create(&admin)

	// Create minimal config
	cfg := &config.Config{}
	cfg.Security.SecretKey = "test_secret_key_12345678901234567890123456789012"
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 5000
	cfg.Upload.Folder = "uploads"
	cfg.Upload.MaxSize = 50 * 1024 * 1024
	cfg.Upload.Subdirs = map[string]string{
		"avatars":       "avatars",
		"room_avatars":  "room_avatars",
		"channel_icons": "channel_icons",
		"files":         "files",
		"music":         "music",
		"videos":        "videos",
	}
	cfg.Session.LifetimeDays = 30
	cfg.Session.CookieName = "boxchat_session"
	cfg.Session.HTTPOnly = true
	cfg.Session.SameSite = "Lax"
	cfg.Session.Secure = false
	cfg.RememberCookie.DurationDays = 30
	cfg.RememberCookie.Name = "boxchat_remember"
	cfg.RememberCookie.HTTPOnly = true
	cfg.RememberCookie.SameSite = "Lax"
	cfg.RememberCookie.Secure = false
	cfg.Giphy.APIKey = ""
	cfg.RootDir = t.TempDir()
	cfg.UploadDir = cfg.RootDir
	cfg.DBPath = dbPath

	// Set global DB for code that uses database.DB directly
	oldDB := database.DB
	database.DB = db

	// Cleanup function
	cleanup := func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
		database.DB = oldDB
		os.Unsetenv("DATABASE_PATH")
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("ADMIN_PASSWORD")
	}

	return cfg, cleanup
}

// SetupTestDBNoAdmin creates an in-memory SQLite database without admin user.
// Useful for tests that need completely clean state.
func SetupTestDBNoAdmin(t *testing.T) (*config.Config, *gorm.DB, func()) {
	t.Helper()

	dbPath := fmt.Sprintf("file:mem_%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	db.Exec("PRAGMA foreign_keys = ON")

	// Minimal migrations - only user and room for basic tests
	if err := db.AutoMigrate(
		&models.User{},
		&models.Room{},
		&models.Channel{},
		&models.Member{},
		&models.Role{},
		&models.MemberRole{},
		&models.Message{},
	); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	cfg := &config.Config{}
	cfg.Security.SecretKey = "test_secret_key_12345678901234567890123456789012"
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 5000
	cfg.Upload.Folder = "uploads"
	cfg.Upload.MaxSize = 50 * 1024 * 1024
	cfg.Upload.Subdirs = map[string]string{
		"avatars":       "avatars",
		"room_avatars":  "room_avatars",
		"channel_icons": "channel_icons",
		"files":         "files",
		"music":         "music",
		"videos":        "videos",
	}
	cfg.Session.LifetimeDays = 30
	cfg.Session.CookieName = "boxchat_session"
	cfg.Session.HTTPOnly = true
	cfg.Session.SameSite = "Lax"
	cfg.Session.Secure = false
	cfg.RememberCookie.DurationDays = 30
	cfg.RememberCookie.Name = "boxchat_remember"
	cfg.RememberCookie.HTTPOnly = true
	cfg.RememberCookie.SameSite = "Lax"
	cfg.RememberCookie.Secure = false
	cfg.Giphy.APIKey = ""
	cfg.RootDir = t.TempDir()
	cfg.UploadDir = cfg.RootDir
	cfg.DBPath = dbPath

	oldDB := database.DB
	database.DB = db

	cleanup := func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
		database.DB = oldDB
	}

	return cfg, db, cleanup
}

// CreateTestUser creates a test user in the database
func CreateTestUser(t *testing.T, db *gorm.DB, username string, isSuperuser bool) *models.User {
	t.Helper()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := models.User{
		Username:       username,
		Password:       string(hashedPassword),
		IsSuperuser:    isSuperuser,
		PresenceStatus: "offline",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return &user
}

// CreateTestRoom creates a test room in the database
func CreateTestRoom(t *testing.T, db *gorm.DB, name string, roomType string, ownerID uint) *models.Room {
	t.Helper()

	room := models.Room{
		Name:      name,
		Type:      roomType,
		OwnerID:   &ownerID,
		IsPublic:  true,
		InviteToken: fmt.Sprintf("invite_%s_%d", name, time.Now().UnixNano()),
	}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}
	return &room
}

// CreateTestMember creates a test member in the database
func CreateTestMember(t *testing.T, db *gorm.DB, userID, roomID uint, role string) *models.Member {
	t.Helper()

	member := models.Member{
		UserID: userID,
		RoomID: roomID,
		Role:   role,
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("Failed to create test member: %v", err)
	}
	return &member
}
