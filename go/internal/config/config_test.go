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
	os.Unsetenv("DATABASE_PATH")
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
	if cfg.Database.Path != "instance/boxchat.db" {
		t.Errorf("Default Database.Path = %s, want instance/boxchat.db", cfg.Database.Path)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Default Server.Host = %s, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 5000 {
		t.Errorf("Default Server.Port = %d, want 5000", cfg.Server.Port)
	}
	if cfg.Upload.MaxSize != 50*1024*1024 {
		t.Errorf("Default Upload.MaxSize = %d, want %d", cfg.Upload.MaxSize, 50*1024*1024)
	}
	if cfg.Session.LifetimeDays != 30 {
		t.Errorf("Default Session.LifetimeDays = %d, want 30", cfg.Session.LifetimeDays)
	}
	if cfg.RememberCookie.DurationDays != 30 {
		t.Errorf("Default RememberCookie.DurationDays = %d, want 30", cfg.RememberCookie.DurationDays)
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
	os.Setenv("DATABASE_PATH", "instance/test.db")
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")
	os.Setenv("GIPHY_API_KEY", "test_giphy_key")
	os.Setenv("SERVER_HOST", "0.0.0.0")
	os.Setenv("SERVER_PORT", "8080")
	os.Setenv("MAX_CONTENT_LENGTH", "104857600") // 100MB

	defer func() {
		os.Unsetenv("DATABASE_PATH")
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

	if cfg.Database.Path != "instance/test.db" {
		t.Errorf("Database.Path = %s, want instance/test.db", cfg.Database.Path)
	}
	if cfg.Security.SecretKey != "test_secret_key_12345678901234567890123456789012" {
		t.Errorf("Security.SecretKey = %s, want test_secret_key_...", cfg.Security.SecretKey)
	}
	if cfg.Giphy.APIKey != "test_giphy_key" {
		t.Errorf("Giphy.APIKey = %s, want test_giphy_key", cfg.Giphy.APIKey)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %s, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Upload.MaxSize != 104857600 {
		t.Errorf("Upload.MaxSize = %d, want 104857600", cfg.Upload.MaxSize)
	}
}

func TestLoad_CustomDatabasePath(t *testing.T) {
	os.Setenv("DATABASE_PATH", "custom/path/mydb.db")
	defer os.Unsetenv("DATABASE_PATH")

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
	if cfg.Security.SecretKey == "" {
		t.Error("Load() should generate Security.SecretKey if not provided")
	}
	if len(cfg.Security.SecretKey) != 64 {
		t.Errorf("Generated Security.SecretKey length = %d, want 64", len(cfg.Security.SecretKey))
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
	if cfg.Session.HTTPOnly != true {
		t.Error("Session.HTTPOnly should be true")
	}
	if cfg.Session.SameSite != "Lax" {
		t.Errorf("Session.SameSite = %s, want Lax", cfg.Session.SameSite)
	}
	if cfg.Session.Secure != false {
		t.Error("Session.Secure should be false")
	}
	if cfg.RememberCookie.HTTPOnly != true {
		t.Error("RememberCookie.HTTPOnly should be true")
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

func TestLoad_MaxUploadSizeWarning(t *testing.T) {
	// This test verifies that high upload.max_size is handled
	// The warning is printed to stdout, so we just verify it doesn't crash
	os.Setenv("MAX_CONTENT_LENGTH", "2147483648") // 2GB
	defer os.Unsetenv("MAX_CONTENT_LENGTH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error with high MAX_CONTENT_LENGTH: %v", err)
	}

	if cfg.Upload.MaxSize != 2147483648 {
		t.Errorf("Upload.MaxSize = %d, want 2147483648", cfg.Upload.MaxSize)
	}
}

func TestLoad_EmptyConfigFile(t *testing.T) {
	// Create a temporary empty config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write empty YAML
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
	if cfg.Database.Path != "instance/boxchat.db" {
		t.Errorf("Empty config should use defaults, got %s", cfg.Database.Path)
	}
}

func TestLoad_InvalidYAMLConfig(t *testing.T) {
	// Create a temporary invalid config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write invalid YAML
	if err := os.WriteFile(configPath, []byte("{invalid yaml"), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Change to temp directory
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer func() {
		os.Chdir(origDir)
		os.Unsetenv("SECRET_KEY")
	}()

	// Should return error for invalid YAML
	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error with invalid YAML")
	}
	if !contains(err.Error(), "failed to parse config.yaml") {
		t.Errorf("Error should mention parse failure, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLoad_ValidYAMLConfig(t *testing.T) {
	// Create a temporary valid YAML config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	yamlConfig := `
database:
  path: instance/testdb.db
server:
  host: 192.168.1.1
  port: 8080
security:
  secret_key: test_key_12345678901234567890123456789012
upload:
  folder: custom_uploads
  max_size: 104857600
session:
  lifetime_days: 60
`

	if err := os.WriteFile(configPath, []byte(yamlConfig), 0644); err != nil {
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
		t.Fatalf("Load() error with valid YAML = %v", err)
	}

	if cfg.Database.Path != "instance/testdb.db" {
		t.Errorf("Database.Path = %s, want instance/testdb.db", cfg.Database.Path)
	}
	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("Server.Host = %s, want 192.168.1.1", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Security.SecretKey != "test_key_12345678901234567890123456789012" {
		t.Errorf("Security.SecretKey = %s, want test_key_...", cfg.Security.SecretKey)
	}
	if cfg.Upload.Folder != "custom_uploads" {
		t.Errorf("Upload.Folder = %s, want custom_uploads", cfg.Upload.Folder)
	}
	if cfg.Upload.MaxSize != 104857600 {
		t.Errorf("Upload.MaxSize = %d, want 104857600", cfg.Upload.MaxSize)
	}
	if cfg.Session.LifetimeDays != 60 {
		t.Errorf("Session.LifetimeDays = %d, want 60", cfg.Session.LifetimeDays)
	}
}
