package http

import (
	"boxchat/internal/database"
	"boxchat/internal/handlers/socketio"
	"boxchat/internal/middleware"
	"boxchat/internal/models"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FriendRequestInput struct {
	Username string `json:"username" binding:"required"`
}

type FriendResponseInput struct {
	Status string `json:"status" binding:"required"` // "accept" or "decline"
}

func (h *APIHandler) FriendStatus(c *gin.Context) {
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

	// Check if self
	if uint(targetID) == user.ID {
		c.JSON(http.StatusOK, gin.H{"success": true, "status": "self"})
		return
	}

	// Check if friends
	if AreFriends(user.ID, uint(targetID)) {
		c.JSON(http.StatusOK, gin.H{"success": true, "status": "friends"})
		return
	}

	// Check for pending requests
	var pendingRequest models.FriendRequest
	
	// Incoming request
	result := database.DB.Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
		user.ID, targetID, targetID, user.ID).
		Where("status = ?", "pending").
		First(&pendingRequest)
	
	if result.Error == nil {
		direction := "outgoing"
		if pendingRequest.ToUserID == user.ID {
			direction = "incoming"
		}
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"status":      "pending",
			"direction":   direction,
			"request_id":  pendingRequest.ID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "status": "none"})
}

// AreFriends checks if two users are friends
func AreFriends(userID1, userID2 uint) bool {
	low, high := minMax(userID1, userID2)
	var friendship models.Friendship
	err := database.DB.Where("user_low_id = ? AND user_high_id = ?", low, high).First(&friendship).Error
	return err == nil
}

func (h *APIHandler) RegisterFriendsRoutes(r *gin.Engine) {
	// Friend status
	r.GET("/api/v1/friends/status/:user_id", middleware.Auth(), h.FriendStatus)
	
	// Friend requests
	r.POST("/api/v1/friends/request", middleware.Auth(), h.SendFriendRequest)
	r.GET("/api/v1/friends/requests", middleware.Auth(), h.GetFriendRequests)
	r.POST("/api/v1/friends/requests/:id/respond", middleware.Auth(), h.RespondToFriendRequest)
	r.DELETE("/api/v1/friends/requests/:id", middleware.Auth(), h.CancelFriendRequest)

	// Friends list
	r.GET("/api/v1/friends", middleware.Auth(), h.GetFriends)
	r.DELETE("/api/v1/friends/:id", middleware.Auth(), h.RemoveFriend)

	// DM rooms
	r.POST("/api/v1/dm/:user_id/create", middleware.Auth(), h.CreateDMRoom)
}

func (h *APIHandler) SendFriendRequest(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	var input FriendRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Find target user
	var target models.User
	if err := database.DB.Where("LOWER(username) = LOWER(?)", input.Username).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Can't send request to self
	if target.ID == user.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send friend request to yourself"})
		return
	}
	
	// Check if already friends
	var existingFriendship models.Friendship
	lowID, highID := minMax(user.ID, target.ID)
	if err := database.DB.Where("user_low_id = ? AND user_high_id = ?", lowID, highID).First(&existingFriendship).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Already friends"})
		return
	}
	
	// Check if request already exists
	var existingRequest models.FriendRequest
	if err := database.DB.Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)", 
		user.ID, target.ID, target.ID, user.ID).First(&existingRequest).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Friend request already exists"})
		return
	}
	
	// Create friend request
	request := models.FriendRequest{
		FromUserID: user.ID,
		ToUserID:   target.ID,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	
	if err := database.DB.Create(&request).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"request": gin.H{
			"id":           request.ID,
			"from_user_id": request.FromUserID,
			"to_user_id":   request.ToUserID,
			"status":       request.Status,
		},
	})
}

func (h *APIHandler) GetFriendRequests(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	// Get incoming requests
	var incoming []models.FriendRequest
	database.DB.Where("to_user_id = ? AND status = ?", user.ID, "pending").
		Preload("FromUser").
		Find(&incoming)
	
	// Get outgoing requests
	var outgoing []models.FriendRequest
	database.DB.Where("from_user_id = ? AND status = ?", user.ID, "pending").
		Preload("ToUser").
		Find(&outgoing)
	
	c.JSON(http.StatusOK, gin.H{
		"incoming": mapRequests(incoming),
		"outgoing": mapRequests(outgoing),
	})
}

func (h *APIHandler) RespondToFriendRequest(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)

	requestID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var input FriendResponseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find request
	var request models.FriendRequest
	if err := database.DB.First(&request, requestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	// Only recipient can respond
	if request.ToUserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if request.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "Request already responded"})
		return
	}

	now := time.Now()

	if input.Status == "accept" {
		request.Status = "accepted"
		request.RespondedAt = &now

		// Create friendship
		lowID, highID := minMax(request.FromUserID, request.ToUserID)
		friendship := models.Friendship{
			UserLowID:  lowID,
			UserHighID: highID,
			CreatedAt:  now,
		}

		if err := database.DB.Create(&friendship).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		// Auto-create DM room
		existingDM := RoomQueryForDM(request.FromUserID, request.ToUserID)
		dmRoomID := uint(0)
		if existingDM.ID == 0 {
			// Create new DM
			fromUser, _ := GetUserByID(request.FromUserID)
			toUser, _ := GetUserByID(request.ToUserID)
			
			dm := models.Room{
				Name:     "DM: " + fromUser.Username + " - " + toUser.Username,
				Type:     "dm",
				IsPublic: false,
			}
			database.DB.Create(&dm)
			
			member1 := models.Member{UserID: request.FromUserID, RoomID: dm.ID, Role: "owner"}
			member2 := models.Member{UserID: request.ToUserID, RoomID: dm.ID, Role: "member"}
			database.DB.Create(&member1)
			database.DB.Create(&member2)
			
			channel := models.Channel{RoomID: dm.ID, Name: "messages"}
			database.DB.Create(&channel)
			
			dmRoomID = dm.ID
			
			// Emit new DM created notification to other user
			socketio.EmitNewDMCreated(request.FromUserID, dm.ID, user.Username, user.AvatarURL)
		} else {
			dmRoomID = existingDM.ID
		}
		
		// Emit friend request updated notification
		socketio.EmitFriendRequestUpdated(request.FromUserID, request.ID, "accepted", user.ID, user.Username, dmRoomID)
	} else if input.Status == "decline" {
		request.Status = "declined"
		request.RespondedAt = &now
		
		// Emit friend request updated notification
		socketio.EmitFriendRequestUpdated(request.FromUserID, request.ID, "declined", user.ID, user.Username, 0)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status must be 'accept' or 'decline'"})
		return
	}

	if err := database.DB.Save(&request).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  request.Status,
	})
}

func (h *APIHandler) CancelFriendRequest(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	requestID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}
	
	var request models.FriendRequest
	if err := database.DB.First(&request, requestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}
	
	// Only sender can cancel
	if request.FromUserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}
	
	if request.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot cancel responded request"})
		return
	}
	
	if err := database.DB.Delete(&request).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIHandler) GetFriends(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	var friendships []models.Friendship
	database.DB.Where("user_low_id = ? OR user_high_id = ?", user.ID, user.ID).
		Find(&friendships)
	
	friends := make([]gin.H, 0)
	for _, f := range friendships {
		var friend models.User
		var friendID uint
		
		if f.UserLowID == user.ID {
			friendID = f.UserHighID
		} else {
			friendID = f.UserLowID
		}
		
		if err := database.DB.First(&friend, friendID).Error; err == nil {
			friends = append(friends, gin.H{
				"id":             friend.ID,
				"username":       friend.Username,
				"avatar_url":     friend.AvatarURL,
				"presence_status": friend.PresenceStatus,
				"friend_since":   f.CreatedAt,
			})
		}
	}
	
	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

func (h *APIHandler) RemoveFriend(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	friendID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid friend ID"})
		return
	}
	
	lowID, highID := minMax(user.ID, uint(friendID))
	
	var friendship models.Friendship
	if err := database.DB.Where("user_low_id = ? AND user_high_id = ?", lowID, highID).First(&friendship).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Friendship not found"})
		return
	}
	
	if err := database.DB.Delete(&friendship).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *APIHandler) CreateDMRoom(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	targetID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	
	// Check if users are friends
	var friendship models.Friendship
	lowID, highID := minMax(user.ID, uint(targetID))
	if err := database.DB.Where("user_low_id = ? AND user_high_id = ?", lowID, highID).First(&friendship).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Can only create DM with friends"})
		return
	}
	
	// Check if DM room already exists
	var existingRoom models.Room
	// First check rooms owned by current user
	err = database.DB.Where("type = ? AND owner_id = ?", "dm", user.ID).
		Joins("JOIN members m1 ON m1.room_id = rooms.id AND m1.user_id = ?", user.ID).
		Joins("JOIN members m2 ON m2.room_id = rooms.id AND m2.user_id = ?", targetID).
		First(&existingRoom).Error
	
	// If not found, check rooms owned by target user
	if err != nil || existingRoom.ID == 0 {
		database.DB.Where("type = ? AND owner_id = ?", "dm", targetID).
			Joins("JOIN members m1 ON m1.room_id = rooms.id AND m1.user_id = ?", targetID).
			Joins("JOIN members m2 ON m2.room_id = rooms.id AND m2.user_id = ?", user.ID).
			First(&existingRoom)
	}
	
	if existingRoom.ID != 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"room_id": existingRoom.ID,
			"existing": true,
		})
		return
	}
	
	// Create DM room
	room := models.Room{
		Name:     "Direct Message",
		Type:     "dm",
		IsPublic: false,
		OwnerID:  &user.ID,
	}
	
	if err := database.DB.Create(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Add both users as members
	member1 := models.Member{
		UserID: user.ID,
		RoomID: room.ID,
		Role:   "owner",
	}
	
	member2 := models.Member{
		UserID: uint(targetID),
		RoomID: room.ID,
		Role:   "member",
	}
	
	database.DB.Create(&member1)
	database.DB.Create(&member2)
	
	// Create default channel
	channel := models.Channel{
		Name:     "messages",
		RoomID:   room.ID,
	}
	database.DB.Create(&channel)
	
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"room_id": room.ID,
		"existing": false,
	})
}

func mapRequests(requests []models.FriendRequest) []gin.H {
	result := make([]gin.H, 0)
	for _, r := range requests {
		result = append(result, gin.H{
			"id":           r.ID,
			"from_user_id": r.FromUserID,
			"to_user_id":   r.ToUserID,
			"status":       r.Status,
			"created_at":   r.CreatedAt,
		})
	}
	return result
}

func minMax(a, b uint) (uint, uint) {
	if a < b {
		return a, b
	}
	return b, a
}

// RoomQueryForDM finds existing DM room between two users
func RoomQueryForDM(userID1, userID2 uint) models.Room {
	var room models.Room
	
	// Find DM rooms where both users are members
	database.DB.Table("rooms").
		Joins("JOIN members m1 ON m1.room_id = rooms.id").
		Joins("JOIN members m2 ON m2.room_id = rooms.id").
		Where("rooms.type = ?", "dm").
		Where("m1.user_id = ? AND m2.user_id = ?", userID1, userID2).
		First(&room)
	
	return room
}

// GetUserByID gets user by ID
func GetUserByID(userID uint) (*models.User, error) {
	var user models.User
	err := database.DB.First(&user, userID).Error
	return &user, err
}
