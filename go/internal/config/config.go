package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Database
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	// Server
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`

	// Security
	Security struct {
		SecretKey string `yaml:"secret_key"`
	} `yaml:"security"`

	// Upload
	Upload struct {
		Folder  string            `yaml:"folder"`
		MaxSize int64             `yaml:"max_size"`
		Subdirs map[string]string `yaml:"subdirs"`
		AllowedExtensions struct {
			Images []string `yaml:"images"`
			Music  []string `yaml:"music"`
			Video  []string `yaml:"video"`
			Files  []string `yaml:"files"`
		} `yaml:"allowed_extensions"`
	} `yaml:"upload"`

	// Session
	Session struct {
		LifetimeDays int    `yaml:"lifetime_days"`
		CookieName   string `yaml:"cookie_name"`
		HTTPOnly     bool   `yaml:"http_only"`
		SameSite     string `yaml:"same_site"`
		Secure       bool   `yaml:"secure"`
	} `yaml:"session"`

	// Remember cookie
	RememberCookie struct {
		DurationDays int    `yaml:"duration_days"`
		Name         string `yaml:"name"`
		HTTPOnly     bool   `yaml:"http_only"`
		SameSite     string `yaml:"same_site"`
		Secure       bool   `yaml:"secure"`
	} `yaml:"remember_cookie"`

	// Giphy
	Giphy struct {
		APIKey string `yaml:"api_key"`
	} `yaml:"giphy"`

	// Computed
	PermanentSessionLifetime time.Duration
	RememberCookieDuration time.Duration
	RootDir                  string
	UploadDir                string
	DBPath                   string
	ServerHost               string
	ServerPort               string
	SecretKey                string
	GiphyAPIKey              string
	AllowedExtensions        []string
	ImageExtensions          []string
	MusicExtensions          []string
	VideoExtensions          []string
}

var Global *Config

// generateSecretKey generates a secure random 32-byte hex string (64 characters)
func generateSecretKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("[CONFIG] CRITICAL: Failed to generate secure random key: %v. Please check system entropy.", err))
	}
	return hex.EncodeToString(bytes)
}

func Load() (*Config, error) {
	// Get root directory
	rootDir, err := os.Getwd()
	if err != nil {
		execPath, _ := os.Executable()
		goDir := filepath.Dir(execPath)
		rootDir = filepath.Dir(goDir)
	}

	cfg := &Config{
		RootDir: rootDir,
	}

	// Defaults
	cfg.Database.Path = "instance/boxchat.db"
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 5000
	cfg.Security.SecretKey = ""
	cfg.Upload.Folder = "uploads"
	cfg.Upload.MaxSize = 50 * 1024 * 1024 // 50MB
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

	// Try to load config.yaml
	configPath := filepath.Join(rootDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
		}
		fmt.Printf("[CONFIG] ✓ Loaded config.yaml\n")

		// Validate max_size
		if cfg.Upload.MaxSize > 1024*1024*1024 {
			fmt.Printf("[CONFIG] ⚠️  WARNING: upload.max_size (%d bytes) is dangerously high! Consider 50MB or less.\n", cfg.Upload.MaxSize)
		}
	} else {
		fmt.Printf("[CONFIG] ⚠️  config.yaml not found, using defaults\n")
	}

	// Override with environment variables (ENV takes precedence)
	if env := os.Getenv("DATABASE_PATH"); env != "" {
		cfg.Database.Path = env
	}
	if env := os.Getenv("SERVER_HOST"); env != "" {
		cfg.Server.Host = env
	}
	if env := os.Getenv("SERVER_PORT"); env != "" {
		fmt.Sscanf(env, "%d", &cfg.Server.Port)
	}
	if env := os.Getenv("SECRET_KEY"); env != "" {
		cfg.Security.SecretKey = env
	}
	if env := os.Getenv("GIPHY_API_KEY"); env != "" {
		cfg.Giphy.APIKey = env
	}
	if env := os.Getenv("UPLOAD_FOLDER"); env != "" {
		cfg.Upload.Folder = env
	}
	if env := os.Getenv("MAX_CONTENT_LENGTH"); env != "" {
		var maxLen int64
		fmt.Sscanf(env, "%d", &maxLen)
		cfg.Upload.MaxSize = maxLen
	}

	// Generate secret key if not provided
	if cfg.Security.SecretKey == "" {
		cfg.Security.SecretKey = generateSecretKey()
		fmt.Println("[CONFIG] ⚠️  No secret_key in config. Generated random key for this session.")
		fmt.Println("[CONFIG] ⚠️  Please set secret_key in config.yaml for production!")
	}

	// Compute durations
	cfg.PermanentSessionLifetime = time.Duration(cfg.Session.LifetimeDays) * 24 * time.Hour
	cfg.RememberCookieDuration = time.Duration(cfg.RememberCookie.DurationDays) * 24 * time.Hour

	// Compute paths
	cfg.UploadDir = filepath.Join(rootDir, cfg.Upload.Folder)
	
	// DB path
	if filepath.IsAbs(cfg.Database.Path) {
		cfg.DBPath = cfg.Database.Path
	} else {
		cfg.DBPath = filepath.Join(rootDir, cfg.Database.Path)
	}

	// Flatten for compatibility
	cfg.ServerHost = cfg.Server.Host
	cfg.ServerPort = fmt.Sprintf("%d", cfg.Server.Port)
	cfg.SecretKey = cfg.Security.SecretKey
	cfg.GiphyAPIKey = cfg.Giphy.APIKey

	// Allowed extensions (hardcoded for security)
	cfg.ImageExtensions = []string{"png", "jpg", "jpeg", "gif", "webp"}
	cfg.MusicExtensions = []string{"mp3", "ogg", "flac", "wav"}
	cfg.VideoExtensions = []string{"mp4", "webm", "mov", "avi", "mkv"}
	filesExtensions := []string{"txt", "py", "js", "html", "css", "json", "xml", "md", "pdf", "zip", "rar"}
	cfg.AllowedExtensions = append(cfg.AllowedExtensions, cfg.ImageExtensions...)
	cfg.AllowedExtensions = append(cfg.AllowedExtensions, cfg.MusicExtensions...)
	cfg.AllowedExtensions = append(cfg.AllowedExtensions, cfg.VideoExtensions...)
	cfg.AllowedExtensions = append(cfg.AllowedExtensions, filesExtensions...)

	Global = cfg
	return cfg, nil
}
