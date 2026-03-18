package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// generateSecretKey Tests
// ============================================================================

func TestGenerateSecretKey(t *testing.T) {
	key1 := generateSecretKey()
	key2 := generateSecretKey()

	// Verify keys are not empty
	if key1 == "" {
		t.Error("generateSecretKey() should not return empty string")
	}
	if key2 == "" {
		t.Error("generateSecretKey() should not return empty string")
	}

	// Verify keys are unique
	if key1 == key2 {
		t.Error("generateSecretKey() should generate unique keys")
	}

	// Verify key length (64 hex characters from 32 bytes)
	if len(key1) != 64 {
		t.Errorf("generateSecretKey() length = %d, want 64", len(key1))
	}
	if len(key2) != 64 {
		t.Errorf("generateSecretKey() length = %d, want 64", len(key2))
	}

	// Verify keys are valid hex
	for _, c := range key1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateSecretKey() returned invalid hex character: %c", c)
		}
	}
}

// ============================================================================
// Load Tests
// ============================================================================

func TestLoad_Defaults(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("SQLALCHEMY_DATABASE_URI")
	os.Unsetenv("SECRET_KEY")
	os.Unsetenv("GIPHY_API_KEY")
	os.Unsetenv("SERVER_HOST")
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("MAX_CONTENT_LENGTH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil")
	}

	// Verify defaults
	if cfg.SQLAlchemyDatabaseURI != "sqlite:///thecomboxmsgr.db" {
		t.Errorf("Default SQLAlchemyDatabaseURI = %s, want sqlite:///thecomboxmsgr.db", cfg.SQLAlchemyDatabaseURI)
	}
	if cfg.ServerHost != "127.0.0.1" {
		t.Errorf("Default ServerHost = %s, want 127.0.0.1", cfg.ServerHost)
	}
	if cfg.ServerPort != "5000" {
		t.Errorf("Default ServerPort = %s, want 5000", cfg.ServerPort)
	}
	if cfg.MaxContentLength != 50*1024*1024 {
		t.Errorf("Default MaxContentLength = %d, want %d", cfg.MaxContentLength, 50*1024*1024)
	}
	if cfg.PermanentSessionLifetimeDays != 30 {
		t.Errorf("Default PermanentSessionLifetimeDays = %d, want 30", cfg.PermanentSessionLifetimeDays)
	}
	if cfg.RememberCookieDurationDays != 30 {
		t.Errorf("Default RememberCookieDurationDays = %d, want 30", cfg.RememberCookieDurationDays)
	}

	// Verify computed values
	if cfg.PermanentSessionLifetime == 0 {
		t.Error("PermanentSessionLifetime should be computed")
	}
	if cfg.RememberCookieDuration == 0 {
		t.Error("RememberCookieDuration should be computed")
	}
	if cfg.UploadDir == "" {
		t.Error("UploadDir should be computed")
	}
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///test.db")
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")
	os.Setenv("GIPHY_API_KEY", "test_giphy_key")
	os.Setenv("SERVER_HOST", "0.0.0.0")
	os.Setenv("SERVER_PORT", "8080")
	os.Setenv("MAX_CONTENT_LENGTH", "104857600") // 100MB

	defer func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("GIPHY_API_KEY")
		os.Unsetenv("SERVER_HOST")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("MAX_CONTENT_LENGTH")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SQLAlchemyDatabaseURI != "sqlite:///test.db" {
		t.Errorf("SQLAlchemyDatabaseURI = %s, want sqlite:///test.db", cfg.SQLAlchemyDatabaseURI)
	}
	if cfg.SecretKey != "test_secret_key_12345678901234567890123456789012" {
		t.Errorf("SecretKey = %s, want test_secret_key_...", cfg.SecretKey)
	}
	if cfg.GiphyAPIKey != "test_giphy_key" {
		t.Errorf("GiphyAPIKey = %s, want test_giphy_key", cfg.GiphyAPIKey)
	}
	if cfg.ServerHost != "0.0.0.0" {
		t.Errorf("ServerHost = %s, want 0.0.0.0", cfg.ServerHost)
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %s, want 8080", cfg.ServerPort)
	}
	if cfg.MaxContentLength != 104857600 {
		t.Errorf("MaxContentLength = %d, want 104857600", cfg.MaxContentLength)
	}
}

func TestLoad_CustomDatabasePath(t *testing.T) {
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///custom/path/mydb.db")
	defer os.Unsetenv("SQLALCHEMY_DATABASE_URI")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// DBPath should be absolute path to the database file
	if cfg.DBPath == "" {
		t.Error("DBPath should be set for custom database path")
	}
	if filepath.Base(cfg.DBPath) != "mydb.db" {
		t.Errorf("DBPath = %s, want path ending with mydb.db", cfg.DBPath)
	}
}

func TestLoad_GeneratesSecretKey(t *testing.T) {
	os.Unsetenv("SECRET_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should generate a secret key if not provided
	if cfg.SecretKey == "" {
		t.Error("Load() should generate SecretKey if not provided")
	}
	if len(cfg.SecretKey) != 64 {
		t.Errorf("Generated SecretKey length = %d, want 64", len(cfg.SecretKey))
	}
}

func TestLoad_GlobalConfig(t *testing.T) {
	os.Unsetenv("SECRET_KEY")

	_, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify Global is set
	if Global == nil {
		t.Error("Load() should set Global config")
	}
}

// ============================================================================
// Config Structure Tests
// ============================================================================

func TestConfigSessionDefaults(t *testing.T) {
	os.Unsetenv("SECRET_KEY")
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify session defaults from loaded config
	if cfg.SessionCookieHTTPOnly != true {
		t.Error("SessionCookieHTTPOnly should be true")
	}
	if cfg.SessionCookieSameSite != "Lax" {
		t.Errorf("SessionCookieSameSite = %s, want Lax", cfg.SessionCookieSameSite)
	}
	if cfg.SessionCookieSecure != false {
		t.Error("SessionCookieSecure should be false")
	}
	if cfg.RememberCookieHTTPOnly != true {
		t.Error("RememberCookieHTTPOnly should be true")
	}
}

func TestConfigUploadSubdirs(t *testing.T) {
	os.Unsetenv("SECRET_KEY")
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expectedSubdirs := map[string]string{
		"avatars":       "avatars",
		"room_avatars":  "room_avatars",
		"channel_icons": "channel_icons",
		"files":         "files",
		"music":         "music",
		"videos":        "videos",
	}

	for key, expected := range expectedSubdirs {
		if actual, ok := cfg.UploadSubdirs[key]; !ok {
			t.Errorf("UploadSubdirs missing key: %s", key)
		} else if actual != expected {
			t.Errorf("UploadSubdirs[%s] = %s, want %s", key, actual, expected)
		}
	}
}

func TestConfigAllowedExtensions(t *testing.T) {
	os.Unsetenv("SECRET_KEY")
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.AllowedExtensions) == 0 {
		t.Error("AllowedExtensions should not be empty")
	}

	// Check for common extensions
	expectedExts := []string{"png", "jpg", "jpeg", "gif", "mp3", "mp4", "pdf"}
	for _, ext := range expectedExts {
		found := false
		for _, allowed := range cfg.AllowedExtensions {
			if allowed == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllowedExtensions should contain: %s", ext)
		}
	}
}

func TestConfigImageExtensions(t *testing.T) {
	os.Unsetenv("SECRET_KEY")
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expectedImageExts := []string{"png", "jpg", "jpeg", "gif", "webp"}
	for _, ext := range expectedImageExts {
		found := false
		for _, allowed := range cfg.ImageExtensions {
			if allowed == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ImageExtensions should contain: %s", ext)
		}
	}
}

func TestConfigMusicExtensions(t *testing.T) {
	os.Unsetenv("SECRET_KEY")
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expectedMusicExts := []string{"mp3", "ogg", "flac", "wav"}
	for _, ext := range expectedMusicExts {
		found := false
		for _, allowed := range cfg.MusicExtensions {
			if allowed == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("MusicExtensions should contain: %s", ext)
		}
	}
}

func TestConfigVideoExtensions(t *testing.T) {
	os.Unsetenv("SECRET_KEY")
	
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	expectedVideoExts := []string{"mp4", "webm", "mov", "avi", "mkv"}
	for _, ext := range expectedVideoExts {
		found := false
		for _, allowed := range cfg.VideoExtensions {
			if allowed == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("VideoExtensions should contain: %s", ext)
		}
	}
}

// ============================================================================
// Edge Cases Tests
// ============================================================================

func TestLoad_MaxContentLengthWarning(t *testing.T) {
	// This test verifies that high MAX_CONTENT_LENGTH is handled
	// The warning is printed to stdout, so we just verify it doesn't crash
	os.Setenv("MAX_CONTENT_LENGTH", "2147483648") // 2GB
	defer os.Unsetenv("MAX_CONTENT_LENGTH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error with high MAX_CONTENT_LENGTH: %v", err)
	}

	if cfg.MaxContentLength != 2147483648 {
		t.Errorf("MaxContentLength = %d, want 2147483648", cfg.MaxContentLength)
	}
}

func TestLoad_EmptyConfigFile(t *testing.T) {
	// Create a temporary empty config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	
	// Write empty JSON
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Change to temp directory
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer func() {
		os.Chdir(origDir)
		os.Unsetenv("SECRET_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error with empty config = %v", err)
	}

	// Should use defaults
	if cfg.SQLAlchemyDatabaseURI != "sqlite:///thecomboxmsgr.db" {
		t.Errorf("Empty config should use defaults, got %s", cfg.SQLAlchemyDatabaseURI)
	}
}

func TestLoad_InvalidJSONConfig(t *testing.T) {
	// Create a temporary invalid config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	
	// Write invalid JSON
	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Change to temp directory
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer func() {
		os.Chdir(origDir)
		os.Unsetenv("SECRET_KEY")
	}()

	// Should not crash, should use defaults
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error with invalid JSON = %v", err)
	}

	if cfg == nil {
		t.Error("Load() should return config even with invalid JSON")
	}
}
