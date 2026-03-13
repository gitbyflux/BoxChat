package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"boxchat/internal/database"
	"boxchat/internal/handlers/socketio"
	"boxchat/internal/middleware"
	"boxchat/internal/models"
	"boxchat/internal/services"
	"github.com/gin-gonic/gin"
)

type BanUserInput struct {
	Reason       string `json:"reason"`
	BanIP        bool   `json:"ban_ip"`
	Duration     int    `json:"duration"`
	RoomID       *uint  `json:"room_id"`
	DeleteMessages bool `json:"delete_messages"`
}

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

func (h *AdminHandler) RegisterAdminRoutes(r *gin.Engine) {
	admin := r.Group("/api/v1/admin", middleware.Auth())
	{
		// Global admin actions (superuser only)
		admin.POST("/user/:user_id/ban", h.BanUser)
		admin.POST("/user/:user_id/unban", h.UnbanUser)
		admin.GET("/banned_users", h.GetBannedUsers)
		admin.GET("/banned_ips", h.GetBannedIPs)
		
		// Room-based admin actions
		admin.POST("/user/:user_id/kick_from_room/:room_id", h.KickUserFromRoom)
		admin.POST("/user/:user_id/mute_in_room/:room_id", h.MuteUserInRoom)
		admin.POST("/user/:user_id/unmute_in_room/:room_id", h.UnmuteUserInRoom)
		admin.POST("/user/:user_id/promote", h.PromoteUser)
		admin.POST("/user/:user_id/demote", h.DemoteUser)
		admin.POST("/user/:user_id/delete_messages", h.DeleteUserMessages)
		
		// Password management
		admin.POST("/user/:user_id/change_password", h.AdminChangePassword)
	}
	
	// User password change (own password)
	r.POST("/api/v1/user/change_password", middleware.Auth(), h.ChangeOwnPassword)
}

func (h *AdminHandler) BanUser(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var input BanUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminService := services.NewAdminService()

	// If room_id provided, ban in room
	if input.RoomID != nil {
		var duration *int
		if input.Duration > 0 {
			duration = &input.Duration
		}
		
		err := adminService.BanUserInRoom(
			user.ID,
			*input.RoomID,
			uint(userID),
			input.Reason,
			duration,
			input.DeleteMessages,
		)
		if err != nil {
			if err == services.ErrNotEnoughRights {
				c.JSON(http.StatusForbidden, gin.H{"error": "Not enough rights to ban in this room"})
				return
			}
			if err == services.ErrNotInRoom {
				c.JSON(http.StatusNotFound, gin.H{"error": "User is not in the room"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "User banned in room",
			"room_id": *input.RoomID,
		})
		return
	}

	// Global ban (superuser only)
	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can perform global bans"})
		return
	}

	var duration *int
	if input.Duration > 0 {
		duration = &input.Duration
	}

	err = adminService.GlobalBanUser(
		user.ID,
		uint(userID),
		input.Reason,
		input.BanIP,
		duration,
		input.DeleteMessages,
	)
	if err != nil {
		if err == services.ErrCannotBanAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot ban admin"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User globally banned",
	})
}

func (h *AdminHandler) UnbanUser(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can unban users"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Remove global ban
	var target models.User
	if err := database.DB.First(&target, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	target.IsBanned = false
	target.BanReason = ""
	target.BannedAt = nil
	database.DB.Save(&target)

	// Remove room bans
	database.DB.Where("user_id = ?", userID).Delete(&models.RoomBan{})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User unbanned",
	})
}

func (h *AdminHandler) GetBannedUsers(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can view banned users"})
		return
	}

	var bannedUsers []models.User
	if err := database.DB.Where("is_banned = ?", true).Find(&bannedUsers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": bannedUsers,
	})
}

type BannedIPInfo struct {
	Username  string  `json:"username"`
	UserID    uint    `json:"user_id"`
	Reason    string  `json:"reason"`
	BannedAt  *string `json:"banned_at"`
}

func (h *AdminHandler) GetBannedIPs(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can view banned IPs"})
		return
	}

	// Get all banned users
	var bannedUsers []models.User
	if err := database.DB.Where("is_banned = ?", true).Find(&bannedUsers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build banned IPs map
	bannedIPsMap := make(map[string][]BannedIPInfo)
	
	for _, user := range bannedUsers {
		if user.BannedIPs != "" {
			ips := strings.Split(user.BannedIPs, ",")
			for _, ip := range ips {
				ip = strings.TrimSpace(ip)
				if ip != "" {
					bannedAtStr := ""
					if user.BannedAt != nil {
						bannedAtStr = user.BannedAt.Format(time.RFC3339)
					}
					bannedIPsMap[ip] = append(bannedIPsMap[ip], BannedIPInfo{
						Username: user.Username,
						UserID:   user.ID,
						Reason:   user.BanReason,
						BannedAt: &bannedAtStr,
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"banned_ips": bannedIPsMap,
		"total_ips":  len(bannedIPsMap),
	})
}

func (h *AdminHandler) KickUserFromRoom(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	adminService := services.NewAdminService()
	
	// Check permission
	if !adminService.IsRoomAdmin(user.ID, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not enough rights"})
		return
	}

	// Delete memberships
	var memberships []models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, m := range memberships {
		database.DB.Delete(&m)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User kicked from room",
	})
}

func (h *AdminHandler) MuteUserInRoom(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	adminService := services.NewAdminService()
	
	// Check permission
	if !adminService.IsRoomAdmin(user.ID, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not enough rights"})
		return
	}

	var input struct {
		Duration int    `json:"duration"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Duration <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Duration must be positive"})
		return
	}

	err = adminService.MuteUserInRoom(user.ID, uint(roomID), uint(userID), input.Duration, input.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User muted",
	})
}

func (h *AdminHandler) UnmuteUserInRoom(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	adminService := services.NewAdminService()
	
	// Check permission
	if !adminService.IsRoomAdmin(user.ID, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not enough rights"})
		return
	}

	err = adminService.UnmuteUserInRoom(user.ID, uint(roomID), uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User unmuted",
	})
}

func (h *AdminHandler) PromoteUser(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can promote users"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var input struct {
		RoomID uint   `json:"room_id" binding:"required"`
		Role   string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Role != "admin" && input.Role != "owner" && input.Role != "member" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
		return
	}

	adminService := services.NewAdminService()
	err = adminService.PromoteUser(user.ID, input.RoomID, uint(userID), input.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User promoted",
	})
}

func (h *AdminHandler) DemoteUser(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can demote users"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var input struct {
		RoomID uint `json:"room_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminService := services.NewAdminService()
	err = adminService.DemoteUser(user.ID, input.RoomID, uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User demoted",
	})
}

func (h *AdminHandler) DeleteUserMessages(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can delete user messages"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var input struct {
		RoomID *uint `json:"room_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminService := services.NewAdminService()
	
	// Count messages before deletion
	var messageCount int64
	if input.RoomID != nil {
		database.DB.Model(&models.Message{}).
			Joins("JOIN channels ON channels.id = messages.channel_id").
			Where("messages.user_id = ? AND channels.room_id = ?", userID, *input.RoomID).
			Count(&messageCount)
	} else {
		database.DB.Model(&models.Message{}).Where("user_id = ?", userID).Count(&messageCount)
	}
	
	err = adminService.DeleteUserMessages(user.ID, uint(userID), input.RoomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Emit WebSocket notification
	if input.RoomID != nil {
		socketio.EmitBulkMessagesDeleted(uint(userID), *input.RoomID, int(messageCount))
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Messages deleted",
		"deleted": messageCount,
	})
}

func (h *AdminHandler) AdminChangePassword(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superusers can change user passwords"})
		return
	}

	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var input struct {
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminService := services.NewAdminService()
	err = adminService.ChangeUserPassword(user.ID, uint(userID), input.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password changed",
	})
}

func (h *AdminHandler) ChangeOwnPassword(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminService := services.NewAdminService()
	if err := adminService.ChangeOwnPassword(user.ID, input.CurrentPassword, input.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password changed",
	})
}
