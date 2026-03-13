package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RoleInput struct {
	Name                     string   `json:"name"`
	MentionTag               string   `json:"mention_tag"`
	CanBeMentionedByEveryone bool     `json:"can_be_mentioned_by_everyone"`
	Permissions              []string `json:"permissions"`
}

type RoleMentionPermissionInput struct {
	SourceRoleID uint `json:"source_role_id"`
	TargetRoleID uint `json:"target_role_id"`
}

func (h *APIHandler) hasManageRolesPermission(userID, roomID uint) bool {
	roleService := services.NewRoleService()
	return roleService.UserHasPermission(userID, roomID, "manage_roles")
}

func (h *APIHandler) CreateRole(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	var input RoleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate mention tag
	mentionTag := normalizeRoleTag(input.MentionTag)
	if mentionTag == "" {
		mentionTag = normalizeRoleTag(input.Name)
	}

	// Validate permissions
	roleService := services.NewRoleService()
	validPermissions := make([]string, 0)
	for _, perm := range input.Permissions {
		if roleService.IsValidPermission(perm) {
			validPermissions = append(validPermissions, perm)
		}
	}

	permissionsJSON, _ := json.Marshal(validPermissions)

	// Create role
	role := models.Role{
		RoomID:                   uint(roomID),
		Name:                     input.Name,
		MentionTag:               mentionTag,
		IsSystem:                 false,
		CanBeMentionedByEveryone: input.CanBeMentionedByEveryone,
		PermissionsJSON:          string(permissionsJSON),
	}

	if err := database.DB.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"role":    role,
	})
}

func (h *APIHandler) GetRole(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	// Check membership
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", userIDUint, roomID).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get role
	var role models.Role
	if err := database.DB.First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if role.RoomID != uint(roomID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to this room"})
		return
	}

	permissions := parseRolePermissions(role.PermissionsJSON)

	c.JSON(http.StatusOK, gin.H{
		"role": gin.H{
			"id":                       role.ID,
			"room_id":                  role.RoomID,
			"name":                     role.Name,
			"mention_tag":              role.MentionTag,
			"is_system":                role.IsSystem,
			"can_be_mentioned_by_everyone": role.CanBeMentionedByEveryone,
			"permissions":              permissions,
			"created_at":               role.CreatedAt,
		},
	})
}

func (h *APIHandler) UpdateRole(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	// Get role
	var role models.Role
	if err := database.DB.First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if role.RoomID != uint(roomID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to this room"})
		return
	}

	// Cannot edit system roles
	if role.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot edit system roles"})
		return
	}

	var input RoleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update role
	updates := make(map[string]interface{})
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.MentionTag != "" {
		updates["mention_tag"] = normalizeRoleTag(input.MentionTag)
	}
	updates["can_be_mentioned_by_everyone"] = input.CanBeMentionedByEveryone

	if len(input.Permissions) > 0 {
		roleService := services.NewRoleService()
		validPermissions := make([]string, 0)
		for _, perm := range input.Permissions {
			if roleService.IsValidPermission(perm) {
				validPermissions = append(validPermissions, perm)
			}
		}
		permissionsJSON, _ := json.Marshal(validPermissions)
		updates["permissions_json"] = string(permissionsJSON)
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&role).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"role":    role,
	})
}

func (h *APIHandler) DeleteRole(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	// Get role
	var role models.Role
	if err := database.DB.First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if role.RoomID != uint(roomID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to this room"})
		return
	}

	// Cannot delete system roles
	if role.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system roles"})
		return
	}

	// Delete role (member_roles and mention_permissions will be deleted via cascade)
	if err := database.DB.Delete(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) UpdateRolePermissions(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	// Get role
	var role models.Role
	if err := database.DB.First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if role.RoomID != uint(roomID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to this room"})
		return
	}

	var input struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update permissions
	roleService := services.NewRoleService()
	if err := roleService.SetRolePermissions(&role, input.Permissions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"permissions": parseRolePermissions(role.PermissionsJSON),
	})
}

func (h *APIHandler) AddRoleMentionPermission(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	var input RoleMentionPermissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate roles exist in room
	var sourceRole, targetRole models.Role
	if err := database.DB.First(&sourceRole, input.SourceRoleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source role not found"})
		return
	}
	if err := database.DB.First(&targetRole, input.TargetRoleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target role not found"})
		return
	}

	if sourceRole.RoomID != uint(roomID) || targetRole.RoomID != uint(roomID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Roles must belong to this room"})
		return
	}

	// Add permission
	roleService := services.NewRoleService()
	if err := roleService.AddRoleMentionPermission(uint(roomID), input.SourceRoleID, input.TargetRoleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) RemoveRoleMentionPermission(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	var input RoleMentionPermissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Remove permission
	roleService := services.NewRoleService()
	if err := roleService.RemoveRoleMentionPermission(uint(roomID), input.SourceRoleID, input.TargetRoleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) AssignRoleToMember(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	memberUserID, err := strconv.ParseUint(c.Param("member_user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member user ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	// Check if member exists
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", memberUserID, roomID).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	var input struct {
		RoleID uint `json:"role_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role exists in room
	var role models.Role
	if err := database.DB.First(&role, input.RoleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Role not found"})
		return
	}

	if role.RoomID != uint(roomID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not belong to this room"})
		return
	}

	// Check if already has role
	var existingMemberRole models.MemberRole
	if err := database.DB.Where("user_id = ? AND room_id = ? AND role_id = ?", memberUserID, roomID, input.RoleID).First(&existingMemberRole).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Role already assigned",
		})
		return
	}

	// Assign role
	memberRole := models.MemberRole{
		UserID:  uint(memberUserID),
		RoomID:  uint(roomID),
		RoleID:  input.RoleID,
	}

	if err := database.DB.Create(&memberRole).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *APIHandler) RemoveRoleFromMember(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDUint := userID.(uint)

	roomID, err := strconv.ParseUint(c.Param("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	memberUserID, err := strconv.ParseUint(c.Param("member_user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member user ID"})
		return
	}

	roleID, err := strconv.ParseUint(c.Param("role_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role ID"})
		return
	}

	// Check permission
	if !h.hasManageRolesPermission(userIDUint, uint(roomID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to manage roles"})
		return
	}

	// Remove role
	if err := database.DB.Where("user_id = ? AND room_id = ? AND role_id = ?", memberUserID, roomID, roleID).Delete(&models.MemberRole{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// normalizeRoleTag normalizes role mention tag
func normalizeRoleTag(tag string) string {
	if tag == "" {
		return ""
	}
	// Convert to lowercase, replace spaces with underscores
	tag = strings.ToLower(strings.TrimSpace(tag))
	tag = strings.ReplaceAll(tag, " ", "_")
	// Remove invalid characters
	tag = regexp.MustCompile(`[^a-z0-9_-]`).ReplaceAllString(tag, "")
	// Limit length
	if len(tag) > 60 {
		tag = tag[:60]
	}
	return tag
}
