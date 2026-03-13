package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"boxchat/internal/config"
	"boxchat/internal/database"
	httpHandler "boxchat/internal/handlers/http"
	"boxchat/internal/handlers/socketio"
	"boxchat/internal/middleware"
	"boxchat/internal/utils"
	"github.com/gin-gonic/gin"
)

func main() {
	fmt.Println("[SERVER STARTUP] Starting BoxChat (Go version)...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[CONFIG] ✗ Failed to load config: %v", err)
	}
	fmt.Println("[CONFIG] ✓ Configuration loaded successfully")

	// Initialize upload folders
	initUploadFolders(cfg.UploadDir, cfg.UploadSubdirs)

	// Initialize database
	if err := database.Init(cfg); err != nil {
		log.Fatalf("[DATABASE] ✗ Failed to initialize: %v", err)
	}

	// Initialize extensions
	utils.InitExtensions(cfg)

	// Initialize Socket.IO hub
	socketio.InitHub()
	fmt.Println("[SOCKET.IO] ✓ Hub initialized")

	// Create Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Middleware
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())

	// Static files - serve frontend dist
	frontendDist := filepath.Join(cfg.RootDir, "frontend", "dist")
	if _, err := os.Stat(frontendDist); err == nil {
		r.Static("/assets", filepath.Join(frontendDist, "assets"))
		r.StaticFile("/favicon.ico", filepath.Join(frontendDist, "favicon.ico"))
		// Serve index.html for root path
		r.StaticFile("/", filepath.Join(frontendDist, "index.html"))
		fmt.Printf("[SERVER] ✓ Frontend dist: %s\n", frontendDist)
	} else {
		fmt.Printf("[SERVER] ✗ Frontend dist not found: %s\n", frontendDist)
	}

	// Serve uploaded files
	r.Static("/uploads", cfg.UploadDir)
	fmt.Printf("[SERVER] ✓ Upload folder: %s\n", cfg.UploadDir)

	// API routes
	authHandler := httpHandler.NewAuthHandler(cfg)
	apiHandler := httpHandler.NewAPIHandler(cfg)

	// Register routes
	authHandler.RegisterRoutes(r)
	apiHandler.RegisterRoutes(r)
	apiHandler.RegisterFriendsRoutes(r)
	apiHandler.RegisterSearchRoutes(r)
	apiHandler.RegisterChannelsRoutes(r)
	apiHandler.RegisterRoomsRoutes(r)
	apiHandler.RegisterRoomsExtraRoutes(r)
	apiHandler.RegisterMusicRoutes(r)
	apiHandler.RegisterStickersRoutes(r)
	apiHandler.RegisterRolesRoutes(r)
	apiHandler.RegisterAdminRoutes(r)
	apiHandler.RegisterBannerRoutes(r)

	// Socket.IO route
	r.GET("/socket.io", middleware.Auth(), socketio.WSHandler)

	// SPA catch-all - serve index.html for all other routes
	r.NoRoute(func(c *gin.Context) {
		// Check if it's an API path
		if c.Request.URL.Path == "/socket.io" {
			return
		}

		// Try to serve from frontend dist
		indexFile := filepath.Join(frontendDist, "index.html")
		if _, err := os.Stat(indexFile); err == nil {
			c.File(indexFile)
			return
		}

		// Fallback to templates
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// Start server
	addr := cfg.ServerHost + ":" + cfg.ServerPort
	fmt.Printf("[SERVER] ✓ Listening on %s\n", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("[SERVER] ✗ Failed to start: %v", err)
	}
}

func initUploadFolders(uploadDir string, subdirs map[string]string) {
	// Create main upload directory
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[UPLOAD] Failed to create upload dir: %v", err)
	}
	
	// Create subdirectories
	for _, subdir := range subdirs {
		path := filepath.Join(uploadDir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			log.Printf("[UPLOAD] Failed to create subdir %s: %v", subdir, err)
		}
	}
	
	fmt.Println("[UPLOAD] Folders initialized")
}
