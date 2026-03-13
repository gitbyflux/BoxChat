package http

import (
	"crypto/rand"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"boxchat/internal/database"
	"boxchat/internal/middleware"
	"boxchat/internal/utils"
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
	fullPath := filepath.Join(h.cfg.UploadDir, filePath)
	
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	
	c.File(fullPath)
}

func saveFile(c *gin.Context, file *multipart.FileHeader, subfolder string, uploadDir string) (string, error) {
	// Create subfolder if not exists
	folderPath := filepath.Join(uploadDir, subfolder)
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return "", err
	}
	
	// Generate unique filename
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
