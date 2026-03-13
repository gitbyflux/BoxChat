package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	SQLAlchemyDatabaseURI        string        `json:"SQLALCHEMY_DATABASE_URI"`
	SQLAlchemyTrackModifications bool          `json:"SQLALCHEMY_TRACK_MODIFICATIONS"`
	SecretKey                    string        `json:"SECRET_KEY"`
	GiphyAPIKey                  string        `json:"GIPHY_API_KEY"`
	UploadFolder                 string        `json:"UPLOAD_FOLDER"`
	MaxContentLength             int64         `json:"MAX_CONTENT_LENGTH"`
	AllowedExtensions            []string      `json:"ALLOWED_EXTENSIONS"`
	ImageExtensions              []string      `json:"IMAGE_EXTENSIONS"`
	MusicExtensions              []string      `json:"MUSIC_EXTENSIONS"`
	VideoExtensions              []string      `json:"VIDEO_EXTENSIONS"`
	UploadSubdirs                map[string]string `json:"UPLOAD_SUBDIRS"`
	
	// Session settings
	PermanentSessionLifetimeDays int `json:"PERMANENT_SESSION_LIFETIME_DAYS"`
	RememberCookieDurationDays   int `json:"REMEMBER_COOKIE_DURATION_DAYS"`
	SessionCookieName            string `json:"SESSION_COOKIE_NAME"`
	SessionCookieHTTPOnly        bool   `json:"SESSION_COOKIE_HTTPONLY"`
	SessionCookieSameSite        string `json:"SESSION_COOKIE_SAMESITE"`
	SessionCookieSecure          bool   `json:"SESSION_COOKIE_SECURE"`
	RememberCookieName           string `json:"REMEMBER_COOKIE_NAME"`
	RememberCookieHTTPOnly       bool   `json:"REMEMBER_COOKIE_HTTPONLY"`
	RememberCookieSameSite       string `json:"REMEMBER_COOKIE_SAMESITE"`
	RememberCookieSecure         bool   `json:"REMEMBER_COOKIE_SECURE"`
	
	// Server settings
	ServerHost string `json:"-"`
	ServerPort string `json:"-"`
	
	// Computed
	PermanentSessionLifetime time.Duration
	RememberCookieDuration time.Duration
	RootDir                  string
	UploadDir                string
	DBPath                   string
}

var Global *Config

// generateSecretKey generates a secure random 32-byte hex string (64 characters)
func generateSecretKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback if random generation fails
		return "INSECURE_FALLBACK_KEY_CHANGE_IMMEDIATELY"
	}
	return hex.EncodeToString(bytes)
}

func Load() (*Config, error) {
	// Get root directory - use current working directory
	// This assumes the server is run from the project root
	rootDir, err := os.Getwd()
	if err != nil {
		// Fallback: derive from executable path
		execPath, _ := os.Executable()
		goDir := filepath.Dir(execPath)
		rootDir = filepath.Dir(goDir)
	}

	// Load config.json from root
	configPath := filepath.Join(rootDir, "config.json")

	cfg := &Config{
		RootDir: rootDir,
	}

	// Defaults
	cfg.SQLAlchemyDatabaseURI = "sqlite:///thecomboxmsgr.db"
	cfg.SQLAlchemyTrackModifications = false
	cfg.SecretKey = ""  // Will be generated if not provided
	cfg.GiphyAPIKey = ""
	cfg.UploadFolder = "uploads"
	cfg.MaxContentLength = 50 * 1024 * 1024 // 50MB default (reasonable limit)
	cfg.AllowedExtensions = []string{"png", "jpg", "jpeg", "gif", "webp", "mp3", "ogg", "flac", "wav", "midi", "mid", "mp4", "webm", "mov", "avi", "mkv", "txt", "py", "js", "html", "css", "json", "xml", "md", "pdf", "zip", "rar"}
	cfg.ImageExtensions = []string{"png", "jpg", "jpeg", "gif", "webp"}
	cfg.MusicExtensions = []string{"mp3", "ogg", "flac", "wav"}
	cfg.VideoExtensions = []string{"mp4", "webm", "mov", "avi", "mkv"}
	cfg.UploadSubdirs = map[string]string{
		"avatars": "avatars",
		"room_avatars": "room_avatars",
		"channel_icons": "channel_icons",
		"files": "files",
		"music": "music",
		"videos": "videos",
	}
	cfg.PermanentSessionLifetimeDays = 30
	cfg.RememberCookieDurationDays = 30
	cfg.SessionCookieName = "boxchat_session"
	cfg.SessionCookieHTTPOnly = true
	cfg.SessionCookieSameSite = "Lax"
	cfg.SessionCookieSecure = false
	cfg.RememberCookieName = "boxchat_remember"
	cfg.RememberCookieHTTPOnly = true
	cfg.RememberCookieSameSite = "Lax"
	cfg.RememberCookieSecure = false
	cfg.ServerHost = "127.0.0.1"
	cfg.ServerPort = "5000"
	
	// Try to load from config.json
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, cfg)
		
		// Validate and warn about dangerous MAX_CONTENT_LENGTH values
		if cfg.MaxContentLength > 1024*1024*1024 { // > 1GB
			fmt.Printf("[CONFIG] ⚠️  WARNING: MAX_CONTENT_LENGTH (%d bytes) is dangerously high! Consider setting it to 50MB (52428800) or less.\n", cfg.MaxContentLength)
		}
	}

	// Override with environment variables (ENV takes precedence)
	if env := os.Getenv("SQLALCHEMY_DATABASE_URI"); env != "" {
		cfg.SQLAlchemyDatabaseURI = env
	}
	if env := os.Getenv("SECRET_KEY"); env != "" {
		cfg.SecretKey = env
	}
	if env := os.Getenv("GIPHY_API_KEY"); env != "" {
		cfg.GiphyAPIKey = env
	}
	if env := os.Getenv("SERVER_HOST"); env != "" {
		cfg.ServerHost = env
	}
	if env := os.Getenv("SERVER_PORT"); env != "" {
		cfg.ServerPort = env
	}
	if env := os.Getenv("MAX_CONTENT_LENGTH"); env != "" {
		var maxLen int64
		fmt.Sscanf(env, "%d", &maxLen)
		cfg.MaxContentLength = maxLen
	}

	// Generate secret key if not provided
	if cfg.SecretKey == "" {
		cfg.SecretKey = generateSecretKey()
		fmt.Println("[CONFIG] ⚠️  No SECRET_KEY provided. Generated random key for this session.")
		fmt.Println("[CONFIG] ⚠️  Please set SECRET_KEY environment variable for production!")
	}
	
	// Compute durations
	cfg.PermanentSessionLifetime = time.Duration(cfg.PermanentSessionLifetimeDays) * 24 * time.Hour
	cfg.RememberCookieDuration = time.Duration(cfg.RememberCookieDurationDays) * 24 * time.Hour
	
	// Compute paths
	cfg.UploadDir = filepath.Join(rootDir, cfg.UploadFolder)
	
	// Parse SQLite path
	if cfg.SQLAlchemyDatabaseURI != "" {
		if cfg.SQLAlchemyDatabaseURI == "sqlite:///thecomboxmsgr.db" {
			cfg.DBPath = filepath.Join(rootDir, "instance", "thecomboxmsgr.db")
		} else if len(cfg.SQLAlchemyDatabaseURI) > 10 && cfg.SQLAlchemyDatabaseURI[:10] == "sqlite:///" {
			dbFile := cfg.SQLAlchemyDatabaseURI[10:]
			if !filepath.IsAbs(dbFile) {
				cfg.DBPath = filepath.Join(rootDir, dbFile)
			} else {
				cfg.DBPath = dbFile
			}
		}
	}
	
	Global = cfg
	return cfg, nil
}
