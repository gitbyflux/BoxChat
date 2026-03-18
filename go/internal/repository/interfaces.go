// Package repository defines interfaces for database operations
package repository

import (
	"boxchat/internal/models"
	"time"
)

// ============================================================================
// User Repository
// ============================================================================

// UserRepository defines interface for user database operations
type UserRepository interface {
	Create(user *models.User) error
	GetByID(id uint) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByUsernameCaseInsensitive(username string) (*models.User, error)
	Update(user *models.User) error
	Delete(id uint) error
	GetAll() ([]*models.User, error)
	Search(query string, limit int) ([]*models.User, error)
	UpdateLoginInfo(id uint, failedAttempts int, lockoutUntil, lastLoginAt *time.Time) error
}

// ============================================================================
// Room Repository
// ============================================================================

// RoomRepository defines interface for room database operations
type RoomRepository interface {
	Create(room *models.Room) error
	GetByID(id uint) (*models.Room, error)
	GetAll() ([]*models.Room, error)
	Update(room *models.Room) error
	Delete(id uint) error
	GetByToken(token string) (*models.Room, error)
	GetMember(roomID, userID uint) (*models.Member, error)
	GetMembers(roomID uint) ([]*models.Member, error)
	AddMember(member *models.Member) error
	RemoveMember(roomID, userID uint) error
	GetBans(roomID uint) ([]*models.RoomBan, error)
	AddBan(ban *models.RoomBan) error
	RemoveBan(roomID, userID uint) error
	GetByOwner(ownerID uint) ([]*models.Room, error)
}

// ============================================================================
// Channel Repository
// ============================================================================

// ChannelRepository defines interface for channel database operations
type ChannelRepository interface {
	Create(channel *models.Channel) error
	GetByID(id uint) (*models.Channel, error)
	GetByRoom(roomID uint) ([]*models.Channel, error)
	Update(channel *models.Channel) error
	Delete(id uint) error
	GetMessages(channelID uint, limit int, offset int64) ([]*models.Message, error)
	GetByRoomAndName(roomID uint, name string) (*models.Channel, error)
}

// ============================================================================
// Message Repository
// ============================================================================

// MessageRepository defines interface for message database operations
type MessageRepository interface {
	Create(msg *models.Message) error
	GetByID(id uint) (*models.Message, error)
	Update(msg *models.Message) error
	Delete(id uint) error
	GetLastRead(userID, channelID uint) (*models.ReadMessage, error)
	UpdateRead(read *models.ReadMessage) error
	GetByChannel(channelID uint, limit int, offset int64) ([]*models.Message, error)
	AddReaction(reaction *models.MessageReaction) error
	RemoveReaction(reactionID uint) error
}

// ============================================================================
// Friend Repository
// ============================================================================

// FriendRepository defines interface for friend database operations
type FriendRepository interface {
	GetFriends(userID uint) ([]*models.Friendship, error)
	GetRequests(userID uint) ([]*models.FriendRequest, error)
	CreateRequest(req *models.FriendRequest) error
	DeleteRequest(id uint) error
	CreateFriend(friend *models.Friendship) error
	DeleteFriend(userID, friendID uint) error
	GetStatus(userID, otherID uint) (string, error)
	AcceptRequest(id uint) error
	GetByUsers(userID, otherID uint) (*models.Friendship, error)
}

// ============================================================================
// Role Repository
// ============================================================================

// RoleRepository defines interface for role database operations
type RoleRepository interface {
	Create(role *models.Role) error
	GetByID(id uint) (*models.Role, error)
	GetByRoom(roomID uint) ([]*models.Role, error)
	Update(role *models.Role) error
	Delete(id uint) error
	GetMemberRoles(memberID uint) ([]*models.MemberRole, error)
	AddMemberRole(memberID, roleID uint) error
	RemoveMemberRole(memberID, roleID uint) error
	GetDefaultRole(roomID uint) (*models.Role, error)
	GetAdminRole(roomID uint) (*models.Role, error)
	GetOwnerRole(roomID uint) (*models.Role, error)
}

// ============================================================================
// Sticker Repository
// ============================================================================

// StickerRepository defines interface for sticker database operations
type StickerRepository interface {
	CreatePack(pack *models.StickerPack) error
	GetPackByID(id uint) (*models.StickerPack, error)
	GetAllPacks() ([]*models.StickerPack, error)
	UpdatePack(pack *models.StickerPack) error
	DeletePack(id uint) error
	CreateSticker(sticker *models.Sticker) error
	GetStickerByID(id uint) (*models.Sticker, error)
	DeleteSticker(id uint) error
	GetPacksByOwner(ownerID uint) ([]*models.StickerPack, error)
	GetStickersByPack(packID uint) ([]*models.Sticker, error)
}

// ============================================================================
// Music Repository
// ============================================================================

// MusicRepository defines interface for music database operations
type MusicRepository interface {
	Create(track *models.UserMusic) error
	GetByUser(userID uint) ([]*models.UserMusic, error)
	Delete(id uint) error
	GetByID(id uint) (*models.UserMusic, error)
}

// ============================================================================
// Member Repository
// ============================================================================

// MemberRepository defines interface for member database operations
type MemberRepository interface {
	Create(member *models.Member) error
	GetByID(id uint) (*models.Member, error)
	GetByRoomAndUser(roomID, userID uint) (*models.Member, error)
	GetByRoom(roomID uint) ([]*models.Member, error)
	Update(member *models.Member) error
	Delete(id uint) error
	DeleteByRoomAndUser(roomID, userID uint) error
}

// ============================================================================
// Auth Throttle Repository
// ============================================================================

// AuthThrottleRepository defines interface for auth throttle database operations
type AuthThrottleRepository interface {
	GetByIP(ip string) (*models.AuthThrottle, error)
	Create(throttle *models.AuthThrottle) error
	Update(throttle *models.AuthThrottle) error
	Delete(id uint) error
}

// ============================================================================
// Room Ban Repository
// ============================================================================

// RoomBanRepository defines interface for room ban database operations
type RoomBanRepository interface {
	Create(ban *models.RoomBan) error
	GetByID(id uint) (*models.RoomBan, error)
	GetByRoom(roomID uint) ([]*models.RoomBan, error)
	GetByUser(roomID, userID uint) (*models.RoomBan, error)
	Delete(id uint) error
	DeleteByRoomAndUser(roomID, userID uint) error
}

// ============================================================================
// Admin Service Interface
// ============================================================================

// AdminServiceInterface defines interface for admin service operations
type AdminServiceInterface interface {
	IsAdmin(userID uint) bool
	IsRoomAdmin(userID, roomID uint) bool
	CanUserAdminRoom(userID, roomID uint) bool
	CanUserBanMembers(userID, roomID uint) bool
	CanUserKickMembers(userID, roomID uint) bool
	CanUserMuteMembers(userID, roomID uint) bool
	GetBannedUsers() ([]*models.User, error)
	GetBannedIPs() ([]map[string]interface{}, error)
	BanUser(adminID, targetID uint, reason string, duration *time.Duration) error
	UnbanUser(adminID, targetID uint) error
	KickUserFromRoom(adminID, targetID, roomID uint, reason string) error
	MuteUserInRoom(adminID, targetID, roomID uint, duration time.Duration, reason string) error
	UnmuteUserInRoom(adminID, targetID, roomID uint) error
	PromoteUser(adminID, targetID uint) error
	DemoteUser(adminID, targetID uint) error
	DeleteUserMessages(adminID, targetID uint, limit int) (int, error)
}

// ============================================================================
// Moderation Service Interface
// ============================================================================

// ModerationServiceInterface defines interface for moderation service operations
type ModerationServiceInterface interface {
	CanUserModerate(moderatorID, targetID, roomID uint) (bool, error)
	MuteUser(moderatorID, targetID, roomID uint, duration time.Duration, reason string) (string, error)
	UnmuteUser(moderatorID, targetID, roomID uint) (string, error)
	KickUser(moderatorID, targetID, roomID uint, reason string) (string, error)
	BanUser(moderatorID, targetID, roomID uint, duration *time.Duration, reason string) (string, error)
	ParseMentions(content string, roomID, channelID uint) *MentionResult
}

// MentionResult contains parsed mention information
type MentionResult struct {
	MentionedUserIDs []uint `json:"mentioned_user_ids"`
	MentionedRoleIDs []uint `json:"mentioned_role_ids"`
	MentionEveryone  bool   `json:"mention_everyone"`
}

// ============================================================================
// Role Service Interface
// ============================================================================

// RoleServiceInterface defines interface for role service operations
type RoleServiceInterface interface {
	GetRolePermissions(role *models.Role) []string
	SetRolePermissions(role *models.Role, permissions []string) error
	GetUserPermissions(userID, roomID uint) ([]string, error)
	UserHasPermission(userID, roomID uint, permission string) (bool, error)
	GetRoles(roomID uint) ([]*models.Role, error)
	CreateRole(roomID uint, name string, permissions []string) (*models.Role, error)
	DeleteRole(roleID uint) error
	GetMemberRoles(memberID uint) ([]*models.Role, error)
	AddRoleToMember(memberID, roleID uint) error
	RemoveRoleFromMember(memberID, roleID uint) error
	EnsureDefaultRoles(roomID uint) error
	CanUserMentionRole(userID, roomID, roleID uint) (bool, error)
}
