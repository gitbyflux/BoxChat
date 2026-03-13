package http

import (
	"net/http"
	"strconv"
	"boxchat/internal/database"
	"boxchat/internal/handlers/socketio"
	"boxchat/internal/middleware"
	"boxchat/internal/models"
	"boxchat/internal/services"
	"github.com/gin-gonic/gin"
)

func (h *APIHandler) JoinRoomByInvite(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invite token required"})
		return
	}

	// Find room by token
	var room models.Room
	if err := database.DB.Where("invite_token = ?", token).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid invite token"})
		return
	}

	// Check if already member
	var existingMember models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", user.ID, room.ID).First(&existingMember).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  "Already a member",
			"room_id":  room.ID,
			"existing": true,
		})
		return
	}

	// Create membership
	member := models.Member{
		UserID: user.ID,
		RoomID: room.ID,
		Role:   "member",
	}
	database.DB.Create(&member)

	// Assign default roles
	roleService := services.NewRoleService()
	roleService.EnsureUserDefaultRoles(user.ID, room.ID)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Joined room",
		"room_id":  room.ID,
		"existing": false,
	})
}

func (h *APIHandler) LeaveRoom(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Get room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	// Cannot leave DM rooms
	if room.Type == "dm" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot leave DM rooms"})
		return
	}

	// Cannot leave if owner
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", user.ID, roomID).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not a member"})
		return
	}

	if member.Role == "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Owner cannot leave room. Transfer ownership or delete the room."})
		return
	}

	// Delete membership
	database.DB.Delete(&member)

	// Delete member roles
	database.DB.Where("user_id = ? AND room_id = ?", user.ID, roomID).Delete(&models.MemberRole{})
	
	// Emit WebSocket notification to user
	socketio.EmitServerRemoved(user.ID, uint(roomID))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Left room",
	})
}

func (h *APIHandler) DeleteDM(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Get room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	// Check if DM
	if room.Type != "dm" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not a DM room"})
		return
	}

	// Check if member
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", user.ID, roomID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Delete user's membership
	database.DB.Delete(&member)

	// Delete member roles
	database.DB.Where("user_id = ? AND room_id = ?", user.ID, roomID).Delete(&models.MemberRole{})

	// If no members left, delete the room and its messages
	var remainingMembers []models.Member
	if err := database.DB.Where("room_id = ?", roomID).Find(&remainingMembers).Error; err == nil {
		if len(remainingMembers) == 0 {
			// Delete messages
			var channels []models.Channel
			if err := database.DB.Where("room_id = ?", roomID).Find(&channels).Error; err == nil {
				for _, ch := range channels {
					database.DB.Where("channel_id = ?", ch.ID).Delete(&models.Message{})
					database.DB.Delete(&ch)
				}
			}
			// Delete room
			database.DB.Delete(&room)
		}
	}
	
	// Emit WebSocket notification to user
	socketio.EmitServerRemoved(user.ID, uint(roomID))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DM deleted",
	})
}

func (h *APIHandler) GetUserProfile(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	targetID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get target user
	var target models.User
	if err := database.DB.First(&target, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check privacy settings
	if !target.PrivacyListable && target.ID != user.ID && !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "User profile is private"})
		return
	}

	// Build profile
	profile := gin.H{
		"id":              target.ID,
		"username":        target.Username,
		"avatar_url":      target.AvatarURL,
		"bio":             target.Bio,
		"presence_status": target.PresenceStatus,
	}

	// Add last_seen if not hidden
	if !target.HideStatus {
		profile["last_seen"] = target.LastSeen
	}

	c.JSON(http.StatusOK, gin.H{
		"profile": profile,
	})
}

func (h *APIHandler) GetStatistics(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Only superusers can access global statistics
	if !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Count users
	var userCount int64
	database.DB.Model(&models.User{}).Count(&userCount)

	// Count rooms
	var roomCount int64
	database.DB.Model(&models.Room{}).Count(&roomCount)

	// Count messages
	var messageCount int64
	database.DB.Model(&models.Message{}).Count(&messageCount)

	// Count online users
	var onlineCount int64
	database.DB.Model(&models.User{}).Where("presence_status = ?", "online").Count(&onlineCount)

	c.JSON(http.StatusOK, gin.H{
		"statistics": gin.H{
			"total_users":   userCount,
			"total_rooms":   roomCount,
			"total_messages": messageCount,
			"online_users":  onlineCount,
		},
	})
}
