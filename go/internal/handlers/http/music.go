package http

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"boxchat/internal/database"
	"boxchat/internal/middleware"
	"boxchat/internal/models"
	"boxchat/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MusicInput struct {
	Title  string `json:"title" binding:"required"`
	Artist string `json:"artist"`
}

func (h *APIHandler) AddMusic(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get music file
	file, err := c.FormFile("music_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "music file is required"})
		return
	}

	// Validate file type
	if !utils.IsMusicFile(file.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong music format"})
		return
	}

	// Save file
	filename, err := h.saveMusicFile(c, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload error"})
		return
	}

	fileURL := "/uploads/music/" + filename

	// Get title and artist from form
	title := c.PostForm("title")
	if title == "" {
		title = strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	}

	artist := c.PostForm("artist")
	if artist == "" {
		artist = "Unknown artist"
	}

	// Handle cover file
	var coverURL string
	coverFile, err := c.FormFile("cover_file")
	if err == nil && coverFile != nil {
		if utils.IsImageFile(coverFile.Filename) {
			coverFilename, err := h.saveAvatarFile(c, coverFile)
			if err == nil {
				coverURL = "/uploads/avatars/" + coverFilename
			}
		}
	}

	// Create music record
	music := models.UserMusic{
		UserID:   user.ID,
		Title:    title,
		Artist:   artist,
		FileURL:  fileURL,
		CoverURL: coverURL,
	}

	if err := database.DB.Create(&music).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"id":      music.ID,
		"music":   music,
	})
}

func (h *APIHandler) GetUserMusic(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var music []models.UserMusic
	if err := database.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Find(&music).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"music": music,
	})
}

func (h *APIHandler) DeleteMusic(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	musicID, err := strconv.ParseUint(c.Param("music_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid music ID"})
		return
	}

	// Get music
	var music models.UserMusic
	if err := database.DB.First(&music, musicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Music not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check ownership
	if music.UserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access"})
		return
	}

	// Delete music file
	if music.FileURL != "" {
		musicPath := music.FileURL
		if len(musicPath) > 9 && musicPath[:9] == "/uploads/" {
			filename := musicPath[9:]
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}
	}

	// Delete cover file
	if music.CoverURL != "" {
		coverPath := music.CoverURL
		if len(coverPath) > 9 && coverPath[:9] == "/uploads/" {
			filename := coverPath[9:]
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}
	}

	// Delete record
	if err := database.DB.Delete(&music).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) saveMusicFile(c *gin.Context, file *multipart.FileHeader) (string, error) {
	return h.saveFileWithExtension(c, file, "music")
}

func (h *APIHandler) saveAvatarFile(c *gin.Context, file *multipart.FileHeader) (string, error) {
	return h.saveFileWithExtension(c, file, "avatars")
}

func (h *APIHandler) saveFileWithExtension(c *gin.Context, file *multipart.FileHeader, subfolder string) (string, error) {
	// Create subfolder if not exists
	folderPath := filepath.Join(h.cfg.UploadDir, subfolder)
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
