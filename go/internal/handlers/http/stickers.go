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

type StickerPackInput struct {
	Name      string `json:"name" binding:"required"`
	IconEmoji string `json:"icon_emoji"`
}

type StickerInput struct {
	Name string `json:"name" binding:"required"`
}

func (h *APIHandler) CreateStickerPack(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input StickerPackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create sticker pack
	pack := models.StickerPack{
		Name:      input.Name,
		IconEmoji: input.IconEmoji,
		OwnerID:   user.ID,
	}

	if err := database.DB.Create(&pack).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"pack":    pack,
	})
}

func (h *APIHandler) GetUserStickerPacks(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var packs []models.StickerPack
	if err := database.DB.Where("owner_id = ?", user.ID).Preload("Stickers").Find(&packs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"packs": packs,
	})
}

func (h *APIHandler) GetStickerPack(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	packID, err := strconv.ParseUint(c.Param("pack_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pack ID"})
		return
	}

	// Get pack
	var pack models.StickerPack
	if err := database.DB.Preload("Stickers").First(&pack, packID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sticker pack not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check ownership
	if pack.OwnerID != user.ID && !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pack": pack,
	})
}

func (h *APIHandler) UpdateStickerPack(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	packID, err := strconv.ParseUint(c.Param("pack_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pack ID"})
		return
	}

	// Get pack
	var pack models.StickerPack
	if err := database.DB.First(&pack, packID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sticker pack not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check ownership
	if pack.OwnerID != user.ID && !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var input StickerPackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update pack
	updates := make(map[string]interface{})
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.IconEmoji != "" {
		updates["icon_emoji"] = input.IconEmoji
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&pack).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"pack":    pack,
	})
}

func (h *APIHandler) DeleteStickerPack(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	packID, err := strconv.ParseUint(c.Param("pack_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pack ID"})
		return
	}

	// Get pack
	var pack models.StickerPack
	if err := database.DB.First(&pack, packID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sticker pack not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check ownership
	if pack.OwnerID != user.ID && !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Delete sticker files
	var stickers []models.Sticker
	if err := database.DB.Where("pack_id = ?", packID).Find(&stickers).Error; err == nil {
		for _, sticker := range stickers {
			if sticker.FileURL != "" {
				stickerPath := sticker.FileURL
				if len(stickerPath) > 9 && stickerPath[:9] == "/uploads/" {
					filename := stickerPath[9:]
					fullPath := filepath.Join(h.cfg.UploadDir, filename)
					if _, err := os.Stat(fullPath); err == nil {
						os.Remove(fullPath)
					}
				}
			}
		}
	}

	// Delete pack (stickers will be deleted via cascade)
	if err := database.DB.Delete(&pack).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) AddSticker(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	packID, err := strconv.ParseUint(c.Param("pack_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pack ID"})
		return
	}

	// Get pack
	var pack models.StickerPack
	if err := database.DB.First(&pack, packID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sticker pack not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check ownership
	if pack.OwnerID != user.ID && !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get sticker file
	file, err := c.FormFile("sticker_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sticker file is required"})
		return
	}

	// Validate file type (images only)
	if !utils.IsImageFile(file.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sticker must be an image"})
		return
	}

	// Save file
	filename, err := h.saveStickerFile(c, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload error"})
		return
	}

	fileURL := "/uploads/stickers/" + filename

	// Get name from form
	var input StickerInput
	if err := c.ShouldBind(&input); err != nil {
		input.Name = strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	}

	// Create sticker
	sticker := models.Sticker{
		Name:    input.Name,
		FileURL: fileURL,
		PackID:  pack.ID,
		OwnerID: user.ID,
	}

	if err := database.DB.Create(&sticker).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"sticker": sticker,
	})
}

func (h *APIHandler) DeleteSticker(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	stickerID, err := strconv.ParseUint(c.Param("sticker_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sticker ID"})
		return
	}

	// Get sticker
	var sticker models.Sticker
	if err := database.DB.First(&sticker, stickerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sticker not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check ownership
	if sticker.OwnerID != user.ID && !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Delete sticker file
	if sticker.FileURL != "" {
		stickerPath := sticker.FileURL
		if len(stickerPath) > 9 && stickerPath[:9] == "/uploads/" {
			filename := stickerPath[9:]
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}
	}

	// Delete sticker
	if err := database.DB.Delete(&sticker).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) saveStickerFile(c *gin.Context, file *multipart.FileHeader) (string, error) {
	// Create stickers subfolder
	folderPath := filepath.Join(h.cfg.UploadDir, "stickers")
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
