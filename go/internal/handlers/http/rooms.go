package http

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/utils"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoomSettingsInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// hasRoomManagementPermission checks if user can manage room settings
func (h *APIHandler) hasRoomManagementPermission(userID, roomID uint) bool {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return false
	}

	// Superuser can always manage
	if user.IsSuperuser {
		return true
	}

	// Check member role
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).First(&member).Error; err != nil {
		return false
	}

	// Owner and admin can manage
	if member.Role == "owner" || member.Role == "admin" {
		return true
	}

	// Check role permissions
	var memberRoles []models.MemberRole
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).Find(&memberRoles).Error; err != nil {
		return false
	}

	for _, mr := range memberRoles {
		var role models.Role
		if err := database.DB.First(&role, mr.RoleID).Error; err != nil {
			continue
		}

		permissions := parseRolePermissions(role.PermissionsJSON)
		for _, perm := range permissions {
			if perm == "manage_server" {
				return true
			}
		}
	}

	return false
}

// hasRoomDeletePermission checks if user can delete the room
func (h *APIHandler) hasRoomDeletePermission(userID, roomID uint) bool {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return false
	}

	// Superuser can always delete
	if user.IsSuperuser {
		return true
	}

	// Only owner can delete room
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).First(&member).Error; err != nil {
		return false
	}

	return member.Role == "owner"
}

func (h *APIHandler) GetRoomSettings(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasRoomManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage room settings"})
		return
	}

	// Get room with details
	var room models.Room
	if err := database.DB.Preload("Owner").Preload("Members.User").Preload("Channels").First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get banned users
	var bans []models.RoomBan
	if err := database.DB.Where("room_id = ?", roomID).Preload("User").Find(&bans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room": room,
		"bans": bans,
	})
}

func (h *APIHandler) UpdateRoomSettings(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasRoomManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage room settings"})
		return
	}

	// Get room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var input RoomSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update room
	updates := make(map[string]interface{})
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&room).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"room":    room,
	})
}

func (h *APIHandler) DeleteRoomAvatar(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasRoomManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage room settings"})
		return
	}

	// Get room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete avatar file if exists
	if room.AvatarURL != "" {
		avatarPath := room.AvatarURL
		if len(avatarPath) > 9 && avatarPath[:9] == "/uploads/" {
			filename := avatarPath[9:] // Remove "/uploads/"
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}

		// Clear avatar URL
		room.AvatarURL = ""
		database.DB.Save(&room)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) DeleteRoom(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Get room first to check if it exists
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check permission - only owner or superuser can delete
	if !h.hasRoomDeletePermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only room owner can delete the room"})
		return
	}

	// Delete room avatar file if exists
	if room.AvatarURL != "" {
		avatarPath := room.AvatarURL
		if len(avatarPath) > 9 && avatarPath[:9] == "/uploads/" {
			filename := avatarPath[9:]
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}
	}

	// Delete room (cascade will delete channels, messages, members, roles, etc.)
	if err := database.DB.Delete(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) GetRoomBans(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasRoomManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage room bans"})
		return
	}

	// Get bans
	var bans []models.RoomBan
	if err := database.DB.Where("room_id = ?", roomID).Preload("User").Preload("BannedBy").Find(&bans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bans": bans,
	})
}

func (h *APIHandler) UnbanUserFromRoom(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userToUnban, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Check permission
	if !h.hasRoomManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage room bans"})
		return
	}

	// Find ban
	var ban models.RoomBan
	if err := database.DB.Where("room_id = ? AND user_id = ?", roomID, userToUnban).First(&ban).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ban not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete ban
	if err := database.DB.Delete(&ban).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) UploadRoomBanner(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasRoomManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage room settings"})
		return
	}

	// Get room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get banner file
	file, err := c.FormFile("banner_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "banner file is required"})
		return
	}

	// Validate file type (images only)
	if !utils.IsImageFile(file.Filename) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "banner must be an image"})
		return
	}

	// Save file
	filename, err := h.saveFileWithSubfolder(c, file, "room_banners")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload error"})
		return
	}

	bannerURL := "/uploads/room_banners/" + filename

	// Update room
	room.BannerURL = bannerURL
	database.DB.Save(&room)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"banner_url": bannerURL,
	})
}

func (h *APIHandler) DeleteRoomBanner(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasRoomManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage room settings"})
		return
	}

	// Get room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete banner file if exists
	if room.BannerURL != "" {
		bannerPath := room.BannerURL
		if len(bannerPath) > 9 && bannerPath[:9] == "/uploads/" {
			filename := bannerPath[9:]
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}

		// Clear banner URL
		room.BannerURL = ""
		database.DB.Save(&room)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) saveFileWithSubfolder(c *gin.Context, file *multipart.FileHeader, subfolder string) (string, error) {
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
