package http

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChannelInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IconEmoji   string `json:"icon_emoji"`
}

type ChannelPermissionsInput struct {
	WriterRoleIDs []uint `json:"writer_role_ids"`
}

// hasChannelManagementPermission checks if user can manage channels in the room
func (h *APIHandler) hasChannelManagementPermission(userID, roomID uint) bool {
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
			if perm == "manage_channels" {
				return true
			}
		}
	}

	return false
}

// parseRolePermissions parses JSON permissions array from role
func parseRolePermissions(permissionsJSON string) []string {
	if permissionsJSON == "" {
		return []string{}
	}

	var permissions []string
	if err := json.Unmarshal([]byte(permissionsJSON), &permissions); err != nil {
		return []string{}
	}

	return permissions
}

func (h *APIHandler) AddChannel(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasChannelManagementPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage channels"})
		return
	}

	var input ChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify room exists
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create channel
	channel := models.Channel{
		Name:        input.Name,
		RoomID:      uint(roomID),
		Description: input.Description,
		IconEmoji:   input.IconEmoji,
	}

	if err := database.DB.Create(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"channel": channel,
	})
}

func (h *APIHandler) EditChannel(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	channelID, err := strconv.ParseUint(c.Param("channel_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	// Get channel
	var channel models.Channel
	if err := database.DB.First(&channel, channelID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check permission
	if !h.hasChannelManagementPermission(userIDUint, channel.RoomID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage channels"})
		return
	}

	var input ChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update channel
	updates := make(map[string]interface{})
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Description != "" {
		updates["description"] = input.Description
	}
	if input.IconEmoji != "" {
		updates["icon_emoji"] = input.IconEmoji
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&channel).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"channel": channel,
	})
}

func (h *APIHandler) DeleteChannel(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	channelID, err := strconv.ParseUint(c.Param("channel_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	// Get channel
	var channel models.Channel
	if err := database.DB.First(&channel, channelID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check permission
	if !h.hasChannelManagementPermission(userIDUint, channel.RoomID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage channels"})
		return
	}

	// Delete channel (messages will be deleted via cascade)
	if err := database.DB.Delete(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) UpdateChannelPermissions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	channelID, err := strconv.ParseUint(c.Param("channel_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	// Get channel
	var channel models.Channel
	if err := database.DB.First(&channel, channelID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check permission
	if !h.hasChannelManagementPermission(userIDUint, channel.RoomID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage channels"})
		return
	}

	var input ChannelPermissionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role IDs - only allow roles from the same room
	var validRoles []models.Role
	if len(input.WriterRoleIDs) > 0 {
		if err := database.DB.Where("room_id = ? AND id IN ?", channel.RoomID, input.WriterRoleIDs).Find(&validRoles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Build JSON array of valid role IDs - validate each role
	validRoleIDs := make([]uint, 0, len(validRoles))
	validRoleIDSet := make(map[uint]bool)
	for _, role := range validRoles {
		if !validRoleIDSet[role.ID] {
			validRoleIDs = append(validRoleIDs, role.ID)
			validRoleIDSet[role.ID] = true
		}
	}

	// Check if any requested role IDs were invalid
	if len(validRoleIDs) != len(input.WriterRoleIDs) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Some role IDs are invalid or do not belong to this room",
		})
		return
	}

	// Serialize to JSON
	var writerRoleIDsJSON string
	if len(validRoleIDs) == 0 {
		writerRoleIDsJSON = "[]"
	} else {
		jsonBytes, err := json.Marshal(validRoleIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		writerRoleIDsJSON = string(jsonBytes)
	}

	// Update channel
	if err := database.DB.Model(&channel).Update("writer_role_ids_json", writerRoleIDsJSON).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"writer_role_ids": validRoleIDs,
	})
}
