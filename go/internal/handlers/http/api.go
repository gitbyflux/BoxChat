package http

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/handlers/socketio"
	"boxchat/internal/middleware"
	"boxchat/internal/models"
	"boxchat/internal/services"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type APIHandler struct {
	cfg          *config.Config
	giphyService *services.GiphyService
}

func NewAPIHandler(cfg *config.Config) *APIHandler {
	return &APIHandler{
		cfg:          cfg,
		giphyService: services.NewGiphyService(cfg.GiphyAPIKey),
	}
}

func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	// User routes
	r.GET("/api/v1/user/me", middleware.Auth(), h.GetCurrentUser)
	r.PATCH("/api/v1/user/settings", middleware.Auth(), h.UpdateUserSettings)
	r.POST("/api/v1/user/avatar", middleware.Auth(), h.UploadAvatar)
	r.DELETE("/api/v1/user/avatar", middleware.Auth(), h.DeleteUserAvatar)
	r.POST("/api/v1/user/delete", middleware.Auth(), h.DeleteAccount)

	// Rooms
	r.GET("/api/v1/rooms", middleware.Auth(), h.ListRooms)
	r.GET("/api/v1/room/:room_id", middleware.Auth(), h.GetRoom)
	r.POST("/api/v1/room/:room_id/join", middleware.Auth(), h.JoinRoom)
	r.GET("/api/v1/room/:room_id/members", middleware.Auth(), h.GetRoomMembers)
	// Note: /api/v1/room/:room_id/roles is registered in RegisterRolesRoutes

	// Channels
	r.GET("/api/v1/channel/:channel_id/messages", middleware.Auth(), h.GetChannelMessages)
	r.POST("/api/v1/channel/:channel_id/mark_read", middleware.Auth(), h.MarkChannelRead)

	// Messages
	r.POST("/api/v1/message/:message_id/reaction", middleware.Auth(), h.AddReaction)
	r.POST("/api/v1/message/:message_id/delete", middleware.Auth(), h.DeleteMessage)
	r.POST("/api/v1/message/:message_id/edit", middleware.Auth(), h.EditMessage)
	r.POST("/api/v1/message/:message_id/forward", middleware.Auth(), h.ForwardMessage)

	// Reactions
	r.GET("/api/v1/reactions", h.ListReactions)

	// GIFs
	r.GET("/api/v1/gifs/trending", h.GifsTrending)
	r.GET("/api/v1/gifs/search", h.GifsSearch)

	// Uploads
	r.POST("/upload_file", h.UploadFile)
	// Note: /uploads/*filepath is served by static file handler in main.go

	// Channels accessible
	r.GET("/channels/accessible", middleware.Auth(), h.GetAccessibleChannels)
}

// RegisterChannelsRoutes registers channel management routes
func (h *APIHandler) RegisterChannelsRoutes(r *gin.Engine) {
	// Channel management routes
	r.POST("/api/v1/room/:room_id/add_channel", middleware.Auth(), h.AddChannel)
	r.PATCH("/api/v1/channel/:channel_id/edit", middleware.Auth(), h.EditChannel)
	r.DELETE("/api/v1/channel/:channel_id/delete", middleware.Auth(), h.DeleteChannel)
	r.PATCH("/api/v1/channel/:channel_id/permissions", middleware.Auth(), h.UpdateChannelPermissions)
}

// RegisterRoomsRoutes registers room management routes
func (h *APIHandler) RegisterRoomsRoutes(r *gin.Engine) {
	// Room settings routes
	r.GET("/api/v1/room/:room_id/settings", middleware.Auth(), h.GetRoomSettings)
	r.PATCH("/api/v1/room/:room_id/settings", middleware.Auth(), h.UpdateRoomSettings)
	r.POST("/api/v1/room/:room_id/avatar/delete", middleware.Auth(), h.DeleteRoomAvatar)
	r.DELETE("/api/v1/room/:room_id/delete", middleware.Auth(), h.DeleteRoom)
	
	// Room bans
	r.GET("/api/v1/room/:room_id/bans", middleware.Auth(), h.GetRoomBans)
	r.POST("/api/v1/room/:room_id/unban/:user_id", middleware.Auth(), h.UnbanUserFromRoom)
}

// RegisterMusicRoutes registers music library routes
func (h *APIHandler) RegisterMusicRoutes(r *gin.Engine) {
	// Music library routes
	r.POST("/api/v1/music/add", middleware.Auth(), h.AddMusic)
	r.GET("/api/v1/user/music", middleware.Auth(), h.GetUserMusic)
	r.POST("/api/v1/music/:music_id/delete", middleware.Auth(), h.DeleteMusic)
}

// RegisterStickersRoutes registers sticker packs routes
func (h *APIHandler) RegisterStickersRoutes(r *gin.Engine) {
	// Sticker pack routes
	r.POST("/api/v1/sticker_packs", middleware.Auth(), h.CreateStickerPack)
	r.GET("/api/v1/sticker_packs", middleware.Auth(), h.GetUserStickerPacks)
	r.GET("/api/v1/sticker_packs/:pack_id", middleware.Auth(), h.GetStickerPack)
	r.PATCH("/api/v1/sticker_packs/:pack_id", middleware.Auth(), h.UpdateStickerPack)
	r.DELETE("/api/v1/sticker_packs/:pack_id", middleware.Auth(), h.DeleteStickerPack)

	// Sticker routes
	r.POST("/api/v1/sticker_packs/:pack_id/stickers", middleware.Auth(), h.AddSticker)
	r.DELETE("/api/v1/stickers/:sticker_id", middleware.Auth(), h.DeleteSticker)
}

// RegisterRolesRoutes registers role management routes
func (h *APIHandler) RegisterRolesRoutes(r *gin.Engine) {
	roleService := services.NewRoleService()

	// Room roles
	r.GET("/api/v1/room/:room_id/roles", middleware.Auth(), h.GetRoomRoles)
	r.POST("/api/v1/room/:room_id/roles", middleware.Auth(), h.CreateRole)
	r.GET("/api/v1/room/:room_id/roles/:role_id", middleware.Auth(), h.GetRole)
	r.PATCH("/api/v1/room/:room_id/roles/:role_id", middleware.Auth(), h.UpdateRole)
	r.DELETE("/api/v1/room/:room_id/roles/:role_id", middleware.Auth(), h.DeleteRole)

	// Role permissions
	r.PATCH("/api/v1/room/:room_id/roles/:role_id/permissions", middleware.Auth(), h.UpdateRolePermissions)

	// Role mention permissions
	r.POST("/api/v1/room/:room_id/roles/mention_permissions", middleware.Auth(), h.AddRoleMentionPermission)
	r.DELETE("/api/v1/room/:room_id/roles/mention_permissions", middleware.Auth(), h.RemoveRoleMentionPermission)

	// Member roles
	r.POST("/api/v1/room/:room_id/members/:member_user_id/roles", middleware.Auth(), h.AssignRoleToMember)
	r.DELETE("/api/v1/room/:room_id/members/:member_user_id/roles/:role_id", middleware.Auth(), h.RemoveRoleFromMember)

	// Use roleService in handlers
	_ = roleService
}

// RegisterAdminRoutes registers admin panel routes
func (h *APIHandler) RegisterAdminRoutes(r *gin.Engine) {
	adminHandler := NewAdminHandler()
	adminHandler.RegisterAdminRoutes(r)
}

// RegisterRoomsExtraRoutes registers additional room routes
func (h *APIHandler) RegisterRoomsExtraRoutes(r *gin.Engine) {
	// Room invite
	r.POST("/api/v1/room/:room_id/invite", middleware.Auth(), h.CreateRoomInvite)
	r.GET("/api/v1/join/:token", middleware.Auth(), h.JoinRoomByInvite)
	
	// Room leave
	r.POST("/api/v1/room/:room_id/leave", middleware.Auth(), h.LeaveRoom)
	
	// Delete DM
	r.POST("/api/v1/room/:room_id/delete_dm", middleware.Auth(), h.DeleteDM)
	
	// User profile
	r.GET("/api/v1/user/:user_id/profile", middleware.Auth(), h.GetUserProfile)
	
	// Statistics
	r.GET("/api/v1/statistics", middleware.Auth(), h.GetStatistics)
}

// RegisterBannerRoutes registers room banner routes
func (h *APIHandler) RegisterBannerRoutes(r *gin.Engine) {
	r.POST("/api/v1/room/:room_id/banner", middleware.Auth(), h.UploadRoomBanner)
	r.DELETE("/api/v1/room/:room_id/banner/delete", middleware.Auth(), h.DeleteRoomBanner)
}

func (h *APIHandler) CreateRoomInvite(c *gin.Context) {
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

	// Check if user is member
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", user.ID, roomID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if user can create invites
	roleService := services.NewRoleService()
	canInvite := member.Role == "owner" || member.Role == "admin" || roleService.UserHasPermission(user.ID, uint(roomID), "invite_members")
	
	if !canInvite {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to create invites"})
		return
	}

	// Get room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	// Generate invite token if not exists
	if room.InviteToken == "" {
		tokenBytes := make([]byte, 16)
		if _, err := rand.Read(tokenBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}
		room.InviteToken = hex.EncodeToString(tokenBytes)
		database.DB.Save(&room)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"invite_token": room.InviteToken,
		"invite_link":  "/join/" + room.InviteToken,
	})
}

func (h *APIHandler) GetCurrentUser(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"id":             user.ID,
		"username":       user.Username,
		"avatar_url":     user.AvatarURL,
		"bio":            user.Bio,
		"privacy_searchable": user.PrivacySearchable,
		"privacy_listable":   user.PrivacyListable,
		"hide_status":    user.HideStatus,
		"presence_status": user.PresenceStatus,
	})
}

func (h *APIHandler) UpdateUserSettings(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	updates := make(map[string]interface{})
	
	if bio, ok := data["bio"].(string); ok {
		if len(bio) > 300 {
			bio = bio[:300]
		}
		updates["bio"] = bio
	}
	
	if username, ok := data["username"].(string); ok {
		if len(username) < 3 || len(username) > 30 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username should be 3-30 characters"})
			return
		}
		
		// Check if username is taken
		var existing models.User
		if err := database.DB.Where("LOWER(username) = LOWER(?) AND id != ?", username, user.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
			return
		}
		updates["username"] = username
	}
	
	if privacySearchable, ok := data["privacy_searchable"].(bool); ok {
		updates["privacy_searchable"] = privacySearchable
	}
	
	if privacyListable, ok := data["privacy_listable"].(bool); ok {
		updates["privacy_listable"] = privacyListable
	}
	
	if hideStatus, ok := data["hide_status"].(bool); ok {
		updates["hide_status"] = hideStatus
		if hideStatus {
			updates["presence_status"] = "hidden"
		} else if user.PresenceStatus != "away" {
			updates["presence_status"] = "online"
		}
	}
	
	if len(updates) > 0 {
		if err := database.DB.Model(user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIHandler) ListRooms(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var rooms []models.Room
	database.DB.Joins("JOIN members ON members.room_id = rooms.id").
		Where("members.user_id = ?", user.ID).
		Find(&rooms)
	
	c.JSON(http.StatusOK, gin.H{"rooms": rooms})
}

func (h *APIHandler) GetAccessibleChannels(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get all rooms where user is a member with channels preloaded
	var memberships []models.Member
	if err := database.DB.Where("user_id = ?", user.ID).
		Preload("Room").
		Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build channels list with room info
	type ChannelWithRoom struct {
		ChannelID   uint   `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		RoomID      uint   `json:"room_id"`
		RoomName    string `json:"room_name"`
		RoomType    string `json:"room_type"`
	}

	var channels []ChannelWithRoom
	
	// Get all channels for these rooms in a single query
	var roomIDs []uint
	for _, m := range memberships {
		roomIDs = append(roomIDs, m.RoomID)
	}
	
	if len(roomIDs) > 0 {
		var allChannels []models.Channel
		if err := database.DB.Where("room_id IN ?", roomIDs).Find(&allChannels).Error; err == nil {
			// Create a map for quick room lookup
			roomMap := make(map[uint]models.Room)
			for _, m := range memberships {
				roomMap[m.RoomID] = m.Room
			}
			
			// Build result
			for _, ch := range allChannels {
				if room, ok := roomMap[ch.RoomID]; ok {
					channels = append(channels, ChannelWithRoom{
						ChannelID:   ch.ID,
						ChannelName: ch.Name,
						RoomID:      room.ID,
						RoomName:    room.Name,
						RoomType:    room.Type,
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": channels,
	})
}

func (h *APIHandler) GetRoom(c *gin.Context) {
	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}
	
	var room models.Room
	if err := database.DB.Preload("Channels").Preload("Members.User").First(&room, roomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"room": room})
}

func (h *APIHandler) JoinRoom(c *gin.Context) {
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
	
	// Check if already a member
	var existingMember models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", user.ID, roomID).First(&existingMember).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "already_member": true})
		return
	}
	
	// Create membership
	member := models.Member{
		UserID: user.ID,
		RoomID: uint(roomID),
		Role:   "member",
	}
	database.DB.Create(&member)
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIHandler) GetRoomMembers(c *gin.Context) {
	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}
	
	var members []models.Member
	database.DB.Preload("User").Where("room_id = ?", roomID).Find(&members)
	
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *APIHandler) GetRoomRoles(c *gin.Context) {
	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}
	
	var roles []models.Role
	database.DB.Where("room_id = ?", roomID).Find(&roles)
	
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func (h *APIHandler) GetChannelMessages(c *gin.Context) {
	channelID, err := strconv.ParseUint(c.Param("channel_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}
	
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 1
	}
	
	var messages []models.Message
	database.DB.Preload("User").Preload("Reactions.User").
		Where("channel_id = ?", channelID).
		Order("messages.timestamp DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages)
	
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func (h *APIHandler) MarkChannelRead(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	channelID, err := strconv.ParseUint(c.Param("channel_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	// Get last message in channel
	var lastMessage models.Message
	if err := database.DB.Where("channel_id = ?", channelID).
		Order("timestamp DESC").First(&lastMessage).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update or create read record
	var readMsg models.ReadMessage
	result := database.DB.Where("user_id = ? AND channel_id = ?", user.ID, channelID).First(&readMsg)

	if result.Error == gorm.ErrRecordNotFound {
		readMsg = models.ReadMessage{
			UserID:            user.ID,
			ChannelID:         uint(channelID),
			LastReadMessageID: &lastMessage.ID,
		}
		database.DB.Create(&readMsg)
	} else {
		database.DB.Model(&readMsg).Updates(map[string]interface{}{
			"last_read_message_id": lastMessage.ID,
		})
	}
	
	// Emit WebSocket notification
	socketio.EmitReadStatusUpdated(user.ID, user.Username, uint(channelID))
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIHandler) AddReaction(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	messageID, err := strconv.ParseUint(c.Param("message_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}
	
	var data struct {
		Emoji        string `json:"emoji"`
		ReactionType string `json:"reaction_type"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if data.Emoji == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "emoji is required"})
		return
	}
	
	// Check if reaction exists
	var reaction models.MessageReaction
	result := database.DB.Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, user.ID, data.Emoji).First(&reaction)
	
	if result.Error == nil {
		// Toggle off - remove reaction
		database.DB.Delete(&reaction)
		c.JSON(http.StatusOK, gin.H{"success": true, "action": "removed"})
	} else {
		// Add reaction
		reaction = models.MessageReaction{
			MessageID:    uint(messageID),
			UserID:       user.ID,
			Emoji:        data.Emoji,
			ReactionType: data.ReactionType,
		}
		database.DB.Create(&reaction)
		c.JSON(http.StatusOK, gin.H{"success": true, "action": "added"})
	}
}

type ForwardMessageInput struct {
	ChannelID uint `json:"channel_id" binding:"required"`
}

func (h *APIHandler) ForwardMessage(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	messageID, err := strconv.ParseUint(c.Param("message_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	var input ForwardMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get original message
	var message models.Message
	if err := database.DB.First(&message, messageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	// Get target channel
	var channel models.Channel
	if err := database.DB.First(&channel, input.ChannelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	// Check access to target channel
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", user.ID, channel.RoomID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to this channel"})
		return
	}

	// Create forwarded message
	forwardedContent := "Forwarded from " + message.User.Username + ":\n" + message.Content
	newMsg := models.Message{
		Content:     forwardedContent,
		UserID:      user.ID,
		ChannelID:   input.ChannelID,
		MessageType: message.MessageType,
		FileURL:     message.FileURL,
		FileName:    message.FileName,
		FileSize:    message.FileSize,
		Timestamp:   time.Now(),
	}

	if err := database.DB.Create(&newMsg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": newMsg,
	})
}

func (h *APIHandler) DeleteMessage(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	messageID, err := strconv.ParseUint(c.Param("message_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	var message models.Message
	if err := database.DB.First(&message, messageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	// Check ownership or admin
	if message.UserID != user.ID && !user.IsSuperuser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not allowed"})
		return
	}

	channelID := message.ChannelID
	
	database.DB.Delete(&message)
	
	// Emit WebSocket notification
	socketio.EmitMessageDeletedGlobal(uint(messageID), channelID)
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIHandler) EditMessage(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	messageID, err := strconv.ParseUint(c.Param("message_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	var data struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var message models.Message
	if err := database.DB.First(&message, messageID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	if message.UserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not allowed"})
		return
	}

	now := time.Now()
	message.Content = data.Content
	message.EditedAt = &now
	database.DB.Save(&message)
	
	editedAtISO := now.Format(time.RFC3339)
	
	// Emit WebSocket notification
	socketio.EmitMessageEditedGlobal(&message, editedAtISO)
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message})
}

func (h *APIHandler) ListReactions(c *gin.Context) {
	reactions := []string{"👍", "❤️", "😂", "😈", "😢", "🔥", "🎉", "🥲", "✅", "❌"}
	c.JSON(http.StatusOK, gin.H{"reactions": reactions})
}

type DeleteAccountInput struct {
	Password string `json:"password" binding:"required"`
}

func (h *APIHandler) DeleteAccount(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input DeleteAccountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid password"})
		return
	}

	userID := user.ID

	// Use transaction for atomic deletion
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("[API] Panic during account deletion: %v", r)
		}
	}()

	// Delete user's music
	if err := tx.Where("user_id = ?", userID).Delete(&models.UserMusic{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete music"})
		return
	}

	// Delete reactions
	if err := tx.Where("user_id = ?", userID).Delete(&models.MessageReaction{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete reactions"})
		return
	}

	// Delete read messages
	if err := tx.Where("user_id = ?", userID).Delete(&models.ReadMessage{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete read messages"})
		return
	}

	// Delete member roles
	if err := tx.Where("user_id = ?", userID).Delete(&models.MemberRole{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete member roles"})
		return
	}

	// Delete memberships
	if err := tx.Where("user_id = ?", userID).Delete(&models.Member{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete memberships"})
		return
	}

	// Delete messages
	if err := tx.Where("user_id = ?", userID).Delete(&models.Message{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete messages"})
		return
	}

	// Delete sticker packs and stickers
	var stickerPacks []models.StickerPack
	if err := tx.Where("owner_id = ?", userID).Find(&stickerPacks).Error; err == nil {
		for _, pack := range stickerPacks {
			tx.Where("pack_id = ?", pack.ID).Delete(&models.Sticker{})
		}
	}
	if err := tx.Where("owner_id = ?", userID).Delete(&models.StickerPack{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete sticker packs"})
		return
	}

	// Delete friendships
	if err := tx.Where("user_low_id = ? OR user_high_id = ?", userID, userID).Delete(&models.Friendship{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete friendships"})
		return
	}

	// Delete friend requests
	if err := tx.Where("from_user_id = ? OR to_user_id = ?", userID, userID).Delete(&models.FriendRequest{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete friend requests"})
		return
	}

	// Delete room bans
	if err := tx.Where("user_id = ? OR banned_by_id = ?", userID, userID).Delete(&models.RoomBan{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete room bans"})
		return
	}

	// Delete avatar file
	if user.AvatarURL != "" {
		avatarPath := user.AvatarURL
		if len(avatarPath) > 9 && avatarPath[:9] == "/uploads/" {
			filename := avatarPath[9:]
			fullPath := filepath.Join(h.cfg.UploadDir, filename)
			if _, err := os.Stat(fullPath); err == nil {
				os.Remove(fullPath)
			}
		}
	}

	// Delete user account
	if err := tx.Delete(user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit deletion"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) GifsTrending(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	
	resp, err := h.giphyService.GetTrendingGifs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, services.MapGiphyResponse(resp))
}

func (h *APIHandler) GifsSearch(c *gin.Context) {
	query := c.DefaultQuery("q", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "24"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	
	resp, err := h.giphyService.SearchGifs(query, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, services.MapGiphyResponse(resp))
}
