package http

import (
	"boxchat/internal/database"
	"boxchat/internal/middleware"
	"boxchat/internal/utils"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *APIHandler) UploadAvatar(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar file is required"})
		return
	}
	
	// Save file
	filename, err := saveFile(c, file, "avatars", h.cfg.UploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	
	// Update user avatar
	user.AvatarURL = "/uploads/avatars/" + filename
	database.DB.Save(user)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"avatar_url": user.AvatarURL,
	})
}

func (h *APIHandler) DeleteUserAvatar(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Delete avatar file if exists
	if user.AvatarURL != "" && !strings.Contains(user.AvatarURL, "placehold.co") {
		avatarPath := user.AvatarURL
		if len(avatarPath) > 9 && avatarPath[:9] == "/uploads/" {
			filename := avatarPath[9:]
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}

		// Clear avatar URL
		user.AvatarURL = ""
		database.DB.Save(user)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"avatar_url": user.AvatarURL,
	})
}

func (h *APIHandler) UploadFile(c *gin.Context) {
	_, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file"})
		return
	}

	if !utils.AllowedFile(file.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file type not allowed"})
		return
	}

	// Determine subfolder based on file type
	subfolder := "files"
	fileType := "file"

	if utils.IsImageFile(file.Filename) {
		subfolder = "files"
		fileType = "image"
	} else if utils.IsMusicFile(file.Filename) {
		subfolder = "music"
		fileType = "music"
	} else if utils.IsVideoFile(file.Filename) {
		subfolder = "videos"
		fileType = "video"
	}
	
	filename, err := saveFile(c, file, subfolder, h.cfg.UploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error saving file"})
		return
	}
	
	fileURL := "/uploads/" + subfolder + "/" + filename
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"url": fileURL,
		"type": fileType,
		"filename": filename,
	})
}

func (h *APIHandler) ServeUploadedFile(c *gin.Context) {
	filePath := c.Param("filepath")
	
	// Prevent path traversal attacks
	// Clean the path and ensure it doesn't contain ..
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}
	
	// Also check the final resolved path is within upload directory
	fullPath := filepath.Join(h.cfg.UploadDir, cleanPath)
	absUploadDir, err := filepath.Abs(h.cfg.UploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	
	// Ensure the resolved path is within the upload directory
	if !strings.HasPrefix(absFullPath, absUploadDir + string(os.PathSeparator)) && absFullPath != absUploadDir {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(fullPath)
}

func saveFile(c *gin.Context, file *multipart.FileHeader, subfolder string, uploadDir string) (string, error) {
	// Validate filename - reject null bytes and path traversal attempts
	originalFilename := file.Filename
	if strings.Contains(originalFilename, "\x00") {
		return "", errors.New("invalid filename: null byte detected")
	}
	if strings.Contains(originalFilename, "..") {
		return "", errors.New("invalid filename: path traversal detected")
	}
	if strings.Contains(originalFilename, "/") || strings.Contains(originalFilename, "\\") {
		// Extract just the filename without path
		originalFilename = filepath.Base(originalFilename)
		file.Filename = originalFilename
	}

	// Create subfolder if not exists
	folderPath := filepath.Join(uploadDir, subfolder)
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return "", err
	}

	// Open uploaded file for MIME type validation
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Read first 512 bytes for MIME type detection
	buffer := make([]byte, 512)
	if _, err := src.Read(buffer); err != nil {
		return "", err
	}
	// Reset file pointer
	src.Seek(0, 0)

	// Detect MIME type
	contentType := http.DetectContentType(buffer)

	// Validate MIME type based on expected types
	allowedMimeTypes := map[string][]string{
		"avatars":       {"image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp"},
		"files":         {"image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "application/pdf", "text/plain", "application/zip", "application/x-zip-compressed"},
		"music":         {"audio/mpeg", "audio/ogg", "audio/flac", "audio/wav", "audio/x-wav"},
		"videos":        {"video/mp4", "video/webm", "video/quicktime", "video/x-msvideo", "video/x-matroska"},
		"channel_icons": {"image/jpeg", "image/png", "image/gif", "image/webp"},
		"room_avatars":  {"image/jpeg", "image/png", "image/gif", "image/webp"},
	}

	expectedMimes, exists := allowedMimeTypes[subfolder]
	if exists {
		valid := false
		for _, mime := range expectedMimes {
			if contentType == mime {
				valid = true
				break
			}
		}
		if !valid {
			return "", errors.New("invalid file type: " + contentType)
		}
	}

	// Generate unique filename to prevent overwrites and path injection
	ext := strings.ToLower(filepath.Ext(file.Filename))
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	filename := hex.EncodeToString(randomBytes) + ext

	// Full path
	fullPath := filepath.Join(folderPath, filename)

	// Save file
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		return "", err
	}

	return filename, nil
}
