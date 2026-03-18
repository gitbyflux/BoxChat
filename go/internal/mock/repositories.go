// Package mock provides mock implementations for testing
package mock

import (
	"boxchat/internal/models"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// Mock User Repository
// ============================================================================

type UserRepository struct {
	CreateFunc                     func(*models.User) error
	GetByIDFunc                    func(uint) (*models.User, error)
	GetByUsernameFunc              func(string) (*models.User, error)
	GetByUsernameCaseInsensitiveFunc func(string) (*models.User, error)
	UpdateFunc                     func(*models.User) error
	DeleteFunc                     func(uint) error
	GetAllFunc                     func() ([]*models.User, error)
	SearchFunc                     func(string, int) ([]*models.User, error)
	UpdateLoginInfoFunc            func(uint, int, *time.Time, *time.Time) error
}

func (m *UserRepository) Create(user *models.User) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(user)
	}
	return nil
}

func (m *UserRepository) GetByID(id uint) (*models.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *UserRepository) GetByUsername(username string) (*models.User, error) {
	if m.GetByUsernameFunc != nil {
		return m.GetByUsernameFunc(username)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *UserRepository) GetByUsernameCaseInsensitive(username string) (*models.User, error) {
	if m.GetByUsernameCaseInsensitiveFunc != nil {
		return m.GetByUsernameCaseInsensitiveFunc(username)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *UserRepository) Update(user *models.User) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(user)
	}
	return nil
}

func (m *UserRepository) Delete(id uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}

func (m *UserRepository) GetAll() ([]*models.User, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc()
	}
	return []*models.User{}, nil
}

func (m *UserRepository) Search(query string, limit int) ([]*models.User, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(query, limit)
	}
	return []*models.User{}, nil
}

func (m *UserRepository) UpdateLoginInfo(id uint, failedAttempts int, lockoutUntil, lastLoginAt *time.Time) error {
	if m.UpdateLoginInfoFunc != nil {
		return m.UpdateLoginInfoFunc(id, failedAttempts, lockoutUntil, lastLoginAt)
	}
	return nil
}

// ============================================================================
// Mock Room Repository
// ============================================================================

type RoomRepository struct {
	CreateFunc       func(*models.Room) error
	GetByIDFunc      func(uint) (*models.Room, error)
	GetAllFunc       func() ([]*models.Room, error)
	UpdateFunc       func(*models.Room) error
	DeleteFunc       func(uint) error
	GetByTokenFunc   func(string) (*models.Room, error)
	GetMemberFunc    func(uint, uint) (*models.Member, error)
	GetMembersFunc   func(uint) ([]*models.Member, error)
	AddMemberFunc    func(*models.Member) error
	RemoveMemberFunc func(uint, uint) error
	GetBansFunc      func(uint) ([]*models.RoomBan, error)
	AddBanFunc       func(*models.RoomBan) error
	RemoveBanFunc    func(uint, uint) error
	GetByOwnerFunc   func(uint) ([]*models.Room, error)
}

func (m *RoomRepository) Create(room *models.Room) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(room)
	}
	return nil
}

func (m *RoomRepository) GetByID(id uint) (*models.Room, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *RoomRepository) GetAll() ([]*models.Room, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc()
	}
	return []*models.Room{}, nil
}

func (m *RoomRepository) Update(room *models.Room) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(room)
	}
	return nil
}

func (m *RoomRepository) Delete(id uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}

func (m *RoomRepository) GetByToken(token string) (*models.Room, error) {
	if m.GetByTokenFunc != nil {
		return m.GetByTokenFunc(token)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *RoomRepository) GetMember(roomID, userID uint) (*models.Member, error) {
	if m.GetMemberFunc != nil {
		return m.GetMemberFunc(roomID, userID)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *RoomRepository) GetMembers(roomID uint) ([]*models.Member, error) {
	if m.GetMembersFunc != nil {
		return m.GetMembersFunc(roomID)
	}
	return []*models.Member{}, nil
}

func (m *RoomRepository) AddMember(member *models.Member) error {
	if m.AddMemberFunc != nil {
		return m.AddMemberFunc(member)
	}
	return nil
}

func (m *RoomRepository) RemoveMember(roomID, userID uint) error {
	if m.RemoveMemberFunc != nil {
		return m.RemoveMemberFunc(roomID, userID)
	}
	return nil
}

func (m *RoomRepository) GetBans(roomID uint) ([]*models.RoomBan, error) {
	if m.GetBansFunc != nil {
		return m.GetBansFunc(roomID)
	}
	return []*models.RoomBan{}, nil
}

func (m *RoomRepository) AddBan(ban *models.RoomBan) error {
	if m.AddBanFunc != nil {
		return m.AddBanFunc(ban)
	}
	return nil
}

func (m *RoomRepository) RemoveBan(roomID, userID uint) error {
	if m.RemoveBanFunc != nil {
		return m.RemoveBanFunc(roomID, userID)
	}
	return nil
}

func (m *RoomRepository) GetByOwner(ownerID uint) ([]*models.Room, error) {
	if m.GetByOwnerFunc != nil {
		return m.GetByOwnerFunc(ownerID)
	}
	return []*models.Room{}, nil
}

// ============================================================================
// Mock Channel Repository
// ============================================================================

type ChannelRepository struct {
	CreateFunc          func(*models.Channel) error
	GetByIDFunc         func(uint) (*models.Channel, error)
	GetByRoomFunc       func(uint) ([]*models.Channel, error)
	UpdateFunc          func(*models.Channel) error
	DeleteFunc          func(uint) error
	GetMessagesFunc     func(uint, int, int64) ([]*models.Message, error)
	GetByRoomAndNameFunc func(uint, string) (*models.Channel, error)
}

func (m *ChannelRepository) Create(channel *models.Channel) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(channel)
	}
	return nil
}

func (m *ChannelRepository) GetByID(id uint) (*models.Channel, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *ChannelRepository) GetByRoom(roomID uint) ([]*models.Channel, error) {
	if m.GetByRoomFunc != nil {
		return m.GetByRoomFunc(roomID)
	}
	return []*models.Channel{}, nil
}

func (m *ChannelRepository) Update(channel *models.Channel) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(channel)
	}
	return nil
}

func (m *ChannelRepository) Delete(id uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}

func (m *ChannelRepository) GetMessages(channelID uint, limit int, offset int64) ([]*models.Message, error) {
	if m.GetMessagesFunc != nil {
		return m.GetMessagesFunc(channelID, limit, offset)
	}
	return []*models.Message{}, nil
}

func (m *ChannelRepository) GetByRoomAndName(roomID uint, name string) (*models.Channel, error) {
	if m.GetByRoomAndNameFunc != nil {
		return m.GetByRoomAndNameFunc(roomID, name)
	}
	return nil, gorm.ErrRecordNotFound
}

// ============================================================================
// Mock Message Repository
// ============================================================================

type MessageRepository struct {
	CreateFunc       func(*models.Message) error
	GetByIDFunc      func(uint) (*models.Message, error)
	UpdateFunc       func(*models.Message) error
	DeleteFunc       func(uint) error
	GetLastReadFunc  func(uint, uint) (*models.ReadMessage, error)
	UpdateReadFunc   func(*models.ReadMessage) error
	GetByChannelFunc func(uint, int, int64) ([]*models.Message, error)
	AddReactionFunc  func(*models.MessageReaction) error
	RemoveReactionFunc func(uint) error
}

func (m *MessageRepository) Create(msg *models.Message) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(msg)
	}
	return nil
}

func (m *MessageRepository) GetByID(id uint) (*models.Message, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MessageRepository) Update(msg *models.Message) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(msg)
	}
	return nil
}

func (m *MessageRepository) Delete(id uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}

func (m *MessageRepository) GetLastRead(userID, channelID uint) (*models.ReadMessage, error) {
	if m.GetLastReadFunc != nil {
		return m.GetLastReadFunc(userID, channelID)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MessageRepository) UpdateRead(read *models.ReadMessage) error {
	if m.UpdateReadFunc != nil {
		return m.UpdateReadFunc(read)
	}
	return nil
}

func (m *MessageRepository) GetByChannel(channelID uint, limit int, offset int64) ([]*models.Message, error) {
	if m.GetByChannelFunc != nil {
		return m.GetByChannelFunc(channelID, limit, offset)
	}
	return []*models.Message{}, nil
}

func (m *MessageRepository) AddReaction(reaction *models.MessageReaction) error {
	if m.AddReactionFunc != nil {
		return m.AddReactionFunc(reaction)
	}
	return nil
}

func (m *MessageRepository) RemoveReaction(reactionID uint) error {
	if m.RemoveReactionFunc != nil {
		return m.RemoveReactionFunc(reactionID)
	}
	return nil
}

// ============================================================================
// Mock Friend Repository
// ============================================================================

type FriendRepository struct {
	GetFriendsFunc     func(uint) ([]*models.Friendship, error)
	GetRequestsFunc    func(uint) ([]*models.FriendRequest, error)
	CreateRequestFunc  func(*models.FriendRequest) error
	DeleteRequestFunc  func(uint) error
	CreateFriendFunc   func(*models.Friendship) error
	DeleteFriendFunc   func(uint, uint) error
	GetStatusFunc      func(uint, uint) (string, error)
	AcceptRequestFunc  func(uint) error
	GetByUsersFunc     func(uint, uint) (*models.Friendship, error)
}

func (m *FriendRepository) GetFriends(userID uint) ([]*models.Friendship, error) {
	if m.GetFriendsFunc != nil {
		return m.GetFriendsFunc(userID)
	}
	return []*models.Friendship{}, nil
}

func (m *FriendRepository) GetRequests(userID uint) ([]*models.FriendRequest, error) {
	if m.GetRequestsFunc != nil {
		return m.GetRequestsFunc(userID)
	}
	return []*models.FriendRequest{}, nil
}

func (m *FriendRepository) CreateRequest(req *models.FriendRequest) error {
	if m.CreateRequestFunc != nil {
		return m.CreateRequestFunc(req)
	}
	return nil
}

func (m *FriendRepository) DeleteRequest(id uint) error {
	if m.DeleteRequestFunc != nil {
		return m.DeleteRequestFunc(id)
	}
	return nil
}

func (m *FriendRepository) CreateFriend(friend *models.Friendship) error {
	if m.CreateFriendFunc != nil {
		return m.CreateFriendFunc(friend)
	}
	return nil
}

func (m *FriendRepository) DeleteFriend(userID, friendID uint) error {
	if m.DeleteFriendFunc != nil {
		return m.DeleteFriendFunc(userID, friendID)
	}
	return nil
}

func (m *FriendRepository) GetStatus(userID, otherID uint) (string, error) {
	if m.GetStatusFunc != nil {
		return m.GetStatusFunc(userID, otherID)
	}
	return "none", nil
}

func (m *FriendRepository) AcceptRequest(id uint) error {
	if m.AcceptRequestFunc != nil {
		return m.AcceptRequestFunc(id)
	}
	return nil
}

func (m *FriendRepository) GetByUsers(userID, otherID uint) (*models.Friendship, error) {
	if m.GetByUsersFunc != nil {
		return m.GetByUsersFunc(userID, otherID)
	}
	return nil, gorm.ErrRecordNotFound
}

// ============================================================================
// Mock Role Repository
// ============================================================================

type RoleRepository struct {
	CreateFunc           func(*models.Role) error
	GetByIDFunc          func(uint) (*models.Role, error)
	GetByRoomFunc        func(uint) ([]*models.Role, error)
	UpdateFunc           func(*models.Role) error
	DeleteFunc           func(uint) error
	GetMemberRolesFunc   func(uint) ([]*models.MemberRole, error)
	AddMemberRoleFunc    func(uint, uint) error
	RemoveMemberRoleFunc func(uint, uint) error
	GetDefaultRoleFunc   func(uint) (*models.Role, error)
	GetAdminRoleFunc     func(uint) (*models.Role, error)
	GetOwnerRoleFunc     func(uint) (*models.Role, error)
}

func (m *RoleRepository) Create(role *models.Role) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(role)
	}
	return nil
}

func (m *RoleRepository) GetByID(id uint) (*models.Role, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *RoleRepository) GetByRoom(roomID uint) ([]*models.Role, error) {
	if m.GetByRoomFunc != nil {
		return m.GetByRoomFunc(roomID)
	}
	return []*models.Role{}, nil
}

func (m *RoleRepository) Update(role *models.Role) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(role)
	}
	return nil
}

func (m *RoleRepository) Delete(id uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}

func (m *RoleRepository) GetMemberRoles(memberID uint) ([]*models.MemberRole, error) {
	if m.GetMemberRolesFunc != nil {
		return m.GetMemberRolesFunc(memberID)
	}
	return []*models.MemberRole{}, nil
}

func (m *RoleRepository) AddMemberRole(memberID, roleID uint) error {
	if m.AddMemberRoleFunc != nil {
		return m.AddMemberRoleFunc(memberID, roleID)
	}
	return nil
}

func (m *RoleRepository) RemoveMemberRole(memberID, roleID uint) error {
	if m.RemoveMemberRoleFunc != nil {
		return m.RemoveMemberRoleFunc(memberID, roleID)
	}
	return nil
}

func (m *RoleRepository) GetDefaultRole(roomID uint) (*models.Role, error) {
	if m.GetDefaultRoleFunc != nil {
		return m.GetDefaultRoleFunc(roomID)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *RoleRepository) GetAdminRole(roomID uint) (*models.Role, error) {
	if m.GetAdminRoleFunc != nil {
		return m.GetAdminRoleFunc(roomID)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *RoleRepository) GetOwnerRole(roomID uint) (*models.Role, error) {
	if m.GetOwnerRoleFunc != nil {
		return m.GetOwnerRoleFunc(roomID)
	}
	return nil, gorm.ErrRecordNotFound
}

// ============================================================================
// Mock Member Repository
// ============================================================================

type MemberRepository struct {
	CreateFunc           func(*models.Member) error
	GetByIDFunc          func(uint) (*models.Member, error)
	GetByRoomAndUserFunc func(uint, uint) (*models.Member, error)
	GetByRoomFunc        func(uint) ([]*models.Member, error)
	UpdateFunc           func(*models.Member) error
	DeleteFunc           func(uint) error
	DeleteByRoomAndUserFunc func(uint, uint) error
}

func (m *MemberRepository) Create(member *models.Member) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(member)
	}
	return nil
}

func (m *MemberRepository) GetByID(id uint) (*models.Member, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(id)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MemberRepository) GetByRoomAndUser(roomID, userID uint) (*models.Member, error) {
	if m.GetByRoomAndUserFunc != nil {
		return m.GetByRoomAndUserFunc(roomID, userID)
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *MemberRepository) GetByRoom(roomID uint) ([]*models.Member, error) {
	if m.GetByRoomFunc != nil {
		return m.GetByRoomFunc(roomID)
	}
	return []*models.Member{}, nil
}

func (m *MemberRepository) Update(member *models.Member) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(member)
	}
	return nil
}

func (m *MemberRepository) Delete(id uint) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(id)
	}
	return nil
}

func (m *MemberRepository) DeleteByRoomAndUser(roomID, userID uint) error {
	if m.DeleteByRoomAndUserFunc != nil {
		return m.DeleteByRoomAndUserFunc(roomID, userID)
	}
	return nil
}

// ============================================================================
// Helper functions
// ============================================================================

func NewMockUserRepository() *UserRepository {
	return &UserRepository{}
}

func NewMockRoomRepository() *RoomRepository {
	return &RoomRepository{}
}

func NewMockChannelRepository() *ChannelRepository {
	return &ChannelRepository{}
}

func NewMockMessageRepository() *MessageRepository {
	return &MessageRepository{}
}

func NewMockFriendRepository() *FriendRepository {
	return &FriendRepository{}
}

func NewMockRoleRepository() *RoleRepository {
	return &RoleRepository{}
}

func NewMockMemberRepository() *MemberRepository {
	return &MemberRepository{}
}
