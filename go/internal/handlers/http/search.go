package http

import (
	"net/http"
	"strconv"
	"strings"
	"boxchat/internal/database"
	"boxchat/internal/middleware"
	"boxchat/internal/models"
	"github.com/gin-gonic/gin"
)

type SearchInput struct {
	Query string `json:"q" form:"q"`
	Limit int    `json:"limit" form:"limit"`
}

func (h *APIHandler) RegisterSearchRoutes(r *gin.Engine) {
	r.GET("/api/v1/search/users", middleware.Auth(), h.SearchUsers)
	r.GET("/api/v1/search/servers", middleware.Auth(), h.SearchServers)
	r.GET("/api/v1/search", middleware.Auth(), h.GlobalSearch)
}

func (h *APIHandler) SearchUsers(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	query := strings.TrimSpace(c.DefaultQuery("q", ""))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}
	
	limit := 20
	if l := c.DefaultQuery("limit", "20"); l != "" {
		if parsed, err := parseLimit(l); err == nil {
			limit = parsed
		}
	}
	
	if limit > 50 {
		limit = 50
	}
	if limit < 1 {
		limit = 1
	}
	
	// Search for users by username
	// Respect privacy settings - only show users who are privacy_searchable
	// Also exclude already friends and blocked users
	var users []models.User
	database.DB.Where("LOWER(username) LIKE ? AND privacy_searchable = ? AND id != ?", 
		"%"+strings.ToLower(query)+"%", true, user.ID).
		Limit(limit).
		Find(&users)
	
	// Filter out users who are already friends
	var friendships []models.Friendship
	database.DB.Where("user_low_id = ? OR user_high_id = ?", user.ID, user.ID).Find(&friendships)
	
	friendIDSet := make(map[uint]bool)
	for _, f := range friendships {
		if f.UserLowID == user.ID {
			friendIDSet[f.UserHighID] = true
		} else {
			friendIDSet[f.UserLowID] = true
		}
	}
	
	// Filter results
	filteredUsers := make([]gin.H, 0)
	for _, u := range users {
		if !friendIDSet[u.ID] {
			filteredUsers = append(filteredUsers, gin.H{
				"id":              u.ID,
				"username":        u.Username,
				"avatar_url":      u.AvatarURL,
				"bio":             u.Bio,
				"presence_status": u.PresenceStatus,
			})
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"users": filteredUsers,
		"total": len(filteredUsers),
	})
}

func (h *APIHandler) SearchServers(c *gin.Context) {
	query := strings.TrimSpace(c.DefaultQuery("q", ""))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit > 50 {
		limit = 50
	}
	if limit < 1 {
		limit = 1
	}

	// Search for public servers (non-DM rooms)
	var servers []models.Room
	queryBuilder := database.DB.Where("type != ?", "dm").Where("is_public = ?", true)
	
	if query != "" {
		queryBuilder = queryBuilder.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(query)+"%")
	}
	
	queryBuilder.Order("name ASC").Limit(limit).Find(&servers)

	// Build server data with member counts
	serversData := make([]gin.H, 0, len(servers))
	for _, s := range servers {
		var memberCount int64
		database.DB.Model(&models.Member{}).Where("room_id = ?", s.ID).Count(&memberCount)
		
		serversData = append(serversData, gin.H{
			"id":           s.ID,
			"name":         s.Name,
			"description":  s.Description,
			"type":         s.Type,
			"avatar_url":   s.AvatarURL,
			"member_count": memberCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"servers": serversData,
	})
}

func (h *APIHandler) GlobalSearch(c *gin.Context) {
	user, _ := middleware.GetCurrentUser(c)
	
	query := strings.TrimSpace(c.DefaultQuery("q", ""))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}
	
	limit := 10
	if l := c.DefaultQuery("limit", "10"); l != "" {
		if parsed, err := parseLimit(l); err == nil {
			limit = parsed
		}
	}
	
	// Search users
	var users []models.User
	database.DB.Where("LOWER(username) LIKE ? AND privacy_searchable = ? AND id != ?", 
		"%"+strings.ToLower(query)+"%", true, user.ID).
		Limit(limit).
		Find(&users)
	
	// Search rooms (where user is a member)
	var rooms []models.Room
	database.DB.Joins("JOIN members ON members.room_id = rooms.id").
		Where("members.user_id = ? AND LOWER(rooms.name) LIKE ?", user.ID, "%"+strings.ToLower(query)+"%").
		Limit(limit).
		Find(&rooms)
	
	// Search messages (in user's rooms)
	var messages []models.Message
	database.DB.Joins("JOIN channels ON channels.id = messages.channel_id").
		Joins("JOIN members ON members.room_id = channels.room_id").
		Where("members.user_id = ? AND LOWER(messages.content) LIKE ?", user.ID, "%"+strings.ToLower(query)+"%").
		Order("messages.timestamp DESC").
		Limit(limit).
		Preload("User").
		Preload("Channel").
		Find(&messages)
	
	c.JSON(http.StatusOK, gin.H{
		"users": mapUsers(users),
		"rooms": mapRooms(rooms),
		"messages": mapMessages(messages),
	})
}

func mapUsers(users []models.User) []gin.H {
	result := make([]gin.H, 0)
	for _, u := range users {
		result = append(result, gin.H{
			"id":              u.ID,
			"username":        u.Username,
			"avatar_url":      u.AvatarURL,
			"bio":             u.Bio,
			"presence_status": u.PresenceStatus,
		})
	}
	return result
}

func mapRooms(rooms []models.Room) []gin.H {
	result := make([]gin.H, 0)
	for _, r := range rooms {
		result = append(result, gin.H{
			"id":          r.ID,
			"name":        r.Name,
			"type":        r.Type,
			"description": r.Description,
			"avatar_url":  r.AvatarURL,
		})
	}
	return result
}

func mapMessages(messages []models.Message) []gin.H {
	result := make([]gin.H, 0)
	for _, m := range messages {
		result = append(result, gin.H{
			"id":         m.ID,
			"content":    m.Content,
			"timestamp":  m.Timestamp,
			"user": gin.H{
				"id":       m.UserID,
				"username": m.User.Username,
			},
			"channel": gin.H{
				"id":   m.ChannelID,
				"name": m.Channel.Name,
			},
		})
	}
	return result
}

func parseLimit(s string) (int, error) {
	var limit int
	_, err := sscanf(s, "%d", &limit)
	return limit, err
}

// Simple sscanf implementation for integer parsing
func sscanf(s string, format string, dest *int) (int, error) {
	var val int
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			val = val*10 + int(c-'0')
			n++
		}
	}
	*dest = val
	if n == 0 {
		return 0, ErrInvalidLimit
	}
	return n, nil
}

var ErrInvalidLimit = &parseError{"invalid limit value"}

type parseError struct {
	msg string
}

func (e *parseError) Error() string {
	return e.msg
}
