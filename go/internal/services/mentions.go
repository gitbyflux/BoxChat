package services

import (
	"boxchat/internal/database"
	"boxchat/internal/models"
	"regexp"
	"sort"
	"strings"
)

type MentionData struct {
	MentionEveryone   bool     `json:"mention_everyone"`
	MentionedUserIDs  []uint   `json:"mentioned_user_ids"`
	MentionedUsernames []string `json:"mentioned_usernames"`
	MentionedRoleIDs  []uint   `json:"mentioned_role_ids"`
	MentionedRoleTags []string `json:"mentioned_role_tags"`
	DeniedRoleTags    []string `json:"denied_role_tags"`
}

type MentionService struct{}

func NewMentionService() *MentionService {
	return &MentionService{}
}

// ParseMentions parses @username and @role mentions from content
func (s *MentionService) ParseMentions(content string, roomID uint, userID uint) *MentionData {
	text := content
	if text == "" {
		return &MentionData{}
	}

	// Find all @mentions
	re := regexp.MustCompile(`@([a-zA-Z0-9_-]{2,60})`)
	tokens := re.FindAllStringSubmatch(text, -1)

	if len(tokens) == 0 {
		return &MentionData{}
	}

	// Get all room members for username lookup
	var members []models.Member
	database.DB.Joins("JOIN users ON users.id = members.user_id").
		Where("members.room_id = ?", roomID).
		Preload("User").
		Find(&members)

	usernameToUser := make(map[string]*models.User)
	for _, m := range members {
		if m.User.ID != 0 {
			usernameToUser[strings.ToLower(m.User.Username)] = &m.User
		}
	}

	// Get all room roles
	var roles []models.Role
	database.DB.Where("room_id = ?", roomID).Find(&roles)

	roleTagToRole := make(map[string]*models.Role)
	for i := range roles {
		roleTagToRole[strings.ToLower(roles[i].MentionTag)] = &roles[i]
	}

	// Separate username tokens and role tokens
	usernameTokens := make(map[string]bool)
	roleTokens := make(map[string]bool)

	for _, token := range tokens {
		if len(token) > 1 {
			tag := strings.ToLower(token[1])
			if role, exists := roleTagToRole[tag]; exists {
				roleTokens[tag] = true
				_ = role // use role for permission check
			} else {
				usernameTokens[tag] = true
			}
		}
	}

	// Find mentioned users by username
	mentionedUsers := make([]*models.User, 0)
	for uname := range usernameTokens {
		if user, exists := usernameToUser[uname]; exists {
			mentionedUsers = append(mentionedUsers, user)
		}
	}

	// Check role mention permissions and get allowed roles
	allowedRoles := make([]*models.Role, 0)
	deniedRoleTags := make([]string, 0)

	for tag := range roleTokens {
		role := roleTagToRole[tag]
		if s.canUserMentionRole(userID, roomID, role) {
			allowedRoles = append(allowedRoles, role)
		} else {
			deniedRoleTags = append(deniedRoleTags, role.MentionTag)
		}
	}

	// OPTIMIZATION: Get all member roles in one query instead of N+1
	roleIDs := make([]uint, 0, len(allowedRoles))
	for _, role := range allowedRoles {
		roleIDs = append(roleIDs, role.ID)
	}

	var allMemberRoles []models.MemberRole
	if len(roleIDs) > 0 {
		database.DB.Where("room_id = ? AND role_id IN ?", roomID, roleIDs).Find(&allMemberRoles)
	}

	// Build map of role_id -> user_ids
	roleUserIDs := make(map[uint]bool)
	for _, mr := range allMemberRoles {
		roleUserIDs[mr.UserID] = true
	}

	// Combine all mentioned user IDs
	allMentionedUserIDs := make(map[uint]bool)
	for _, user := range mentionedUsers {
		allMentionedUserIDs[user.ID] = true
	}
	for uid := range roleUserIDs {
		allMentionedUserIDs[uid] = true
	}

	// OPTIMIZATION: Get all usernames in one query instead of N+1
	sortedUserIDs := make([]uint, 0, len(allMentionedUserIDs))
	for uid := range allMentionedUserIDs {
		sortedUserIDs = append(sortedUserIDs, uid)
	}
	sort.Slice(sortedUserIDs, func(i, j int) bool {
		return sortedUserIDs[i] < sortedUserIDs[j]
	})

	// Single query to get all mentioned users
	var mentionedUsersList []models.User
	if len(sortedUserIDs) > 0 {
		database.DB.Where("id IN ?", sortedUserIDs).Find(&mentionedUsersList)
	}

	// Build username map for quick lookup
	userIDToUsername := make(map[uint]string)
	for _, user := range mentionedUsersList {
		userIDToUsername[user.ID] = user.Username
	}

	// Build usernames list in sorted order
	allMentionedUsernames := make([]string, 0, len(sortedUserIDs))
	for _, uid := range sortedUserIDs {
		if username, exists := userIDToUsername[uid]; exists {
			allMentionedUsernames = append(allMentionedUsernames, username)
		}
	}

	// Check for @everyone
	mentionEveryone := false
	for _, role := range allowedRoles {
		if strings.ToLower(role.MentionTag) == "everyone" {
			mentionEveryone = true
			break
		}
	}

	// Build mentioned role IDs and tags
	mentionedRoleIDs := make([]uint, 0, len(allowedRoles))
	mentionedRoleTags := make([]string, 0, len(allowedRoles))
	for _, role := range allowedRoles {
		mentionedRoleIDs = append(mentionedRoleIDs, role.ID)
		mentionedRoleTags = append(mentionedRoleTags, role.MentionTag)
	}

	return &MentionData{
		MentionEveryone:    mentionEveryone,
		MentionedUserIDs:   sortedUserIDs,
		MentionedUsernames: allMentionedUsernames,
		MentionedRoleIDs:   mentionedRoleIDs,
		MentionedRoleTags:  mentionedRoleTags,
		DeniedRoleTags:     deniedRoleTags,
	}
}

// canUserMentionRole checks if user can mention a specific role
func (s *MentionService) canUserMentionRole(userID, roomID uint, targetRole *models.Role) bool {
	if targetRole == nil {
		return false
	}
	
	// If role can be mentioned by everyone, allow
	if targetRole.CanBeMentionedByEveryone {
		return true
	}
	
	// Check if user has a role that can mention this role
	var permissions []models.RoleMentionPermission
	database.DB.Where("room_id = ? AND target_role_id = ?", roomID, targetRole.ID).
		Find(&permissions)
	
	if len(permissions) == 0 {
		// No specific permissions set, check if target role is system role
		if targetRole.IsSystem {
			return true
		}
		return false
	}
	
	// Get user's roles in this room
	userRoleIDs := s.getUserRoleIDs(userID, roomID)
	
	for _, perm := range permissions {
		if userRoleIDs[perm.SourceRoleID] {
			return true
		}
	}
	
	return false
}

// getUserRoleIDs returns map of role IDs for user in room
func (s *MentionService) getUserRoleIDs(userID, roomID uint) map[uint]bool {
	roleIDs := make(map[uint]bool)
	
	var memberRoles []models.MemberRole
	database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).Find(&memberRoles)
	
	for _, mr := range memberRoles {
		roleIDs[mr.RoleID] = true
	}
	
	// Also include member's main role
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", userID, roomID).First(&member).Error; err == nil {
		// Map string roles to potential role IDs if needed
		if member.Role == "owner" || member.Role == "admin" {
			// These are elevated roles
			roleIDs[0] = true // Placeholder for system roles
		}
	}
	
	return roleIDs
}

// GetMentionNotificationUsers returns list of user IDs that should be notified
func (s *MentionService) GetMentionNotificationUsers(mentionData *MentionData, channelID uint) []uint {
	userIDs := make(map[uint]bool)
	
	// Add directly mentioned users
	for _, uid := range mentionData.MentionedUserIDs {
		userIDs[uid] = true
	}
	
	// If @everyone, get all channel members
	if mentionData.MentionEveryone {
		var channel models.Channel
		if err := database.DB.First(&channel, channelID).Error; err == nil {
			var members []models.Member
			database.DB.Where("room_id = ?", channel.RoomID).Find(&members)
			for _, m := range members {
				userIDs[m.UserID] = true
			}
		}
	}
	
	result := make([]uint, 0)
	for uid := range userIDs {
		result = append(result, uid)
	}
	
	return result
}
