package models

import (
	"time"
	"gorm.io/gorm"
)

// Base model
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ============================================================================
// User models
// ============================================================================

type User struct {
	BaseModel
	Username    string `gorm:"uniqueIndex;size:150;not null" json:"username"`
	Password    string `gorm:"size:150;not null" json:"-"`
	
	// Profile info
	Bio       string `gorm:"size:300" json:"bio"`
	AvatarURL string `gorm:"size:300" json:"avatar_url"`
	BirthDate string `gorm:"size:20" json:"birth_date"`
	
	// Privacy settings
	PrivacySearchable bool   `gorm:"default:true" json:"privacy_searchable"`
	PrivacyListable   bool   `gorm:"default:true" json:"privacy_listable"`
	PresenceStatus    string `gorm:"size:20;default:'offline'" json:"presence_status"`
	LastSeen          *time.Time `json:"last_seen"`
	HideStatus        bool   `gorm:"default:false" json:"hide_status"`
	
	// Permissions
	IsSuperuser bool `gorm:"default:false" json:"is_superuser"`
	
	// Ban management
	IsBanned    bool   `gorm:"default:false" json:"is_banned"`
	BannedIPs   string `gorm:"type:text" json:"banned_ips"`
	BanReason   string `gorm:"size:500" json:"ban_reason"`
	BannedAt    *time.Time `json:"banned_at"`
	
	// Authentication hardening
	FailedLoginAttempts int        `gorm:"default:0;not null" json:"-"`
	LockoutUntil        *time.Time `json:"-"`
	LastLoginAt         *time.Time `json:"last_login_at"`
	LastLoginIP         string     `gorm:"size:64" json:"-"`
	
	// Relationships
	Memberships []Member     `gorm:"foreignKey:UserID" json:"-"`
	MusicTracks []UserMusic  `gorm:"foreignKey:UserID" json:"-"`
	Reactions   []MessageReaction `gorm:"foreignKey:UserID" json:"-"`
	ReadMessages []ReadMessage `gorm:"foreignKey:UserID" json:"-"`
	MemberRoles []MemberRole  `gorm:"foreignKey:UserID" json:"-"`
	
	// Friendships
	FriendshipsLow  []Friendship `gorm:"foreignKey:UserLowID" json:"-"`
	FriendshipsHigh []Friendship `gorm:"foreignKey:UserHighID" json:"-"`
	FriendRequestsFrom []FriendRequest `gorm:"foreignKey:FromUserID" json:"-"`
	FriendRequestsTo   []FriendRequest `gorm:"foreignKey:ToUserID" json:"-"`
	
	// Stickers
	StickerPacks []StickerPack `gorm:"foreignKey:OwnerID" json:"-"`
	Stickers     []Sticker     `gorm:"foreignKey:OwnerID" json:"-"`
}

type AuthThrottle struct {
	BaseModel
	IPAddress     string     `gorm:"uniqueIndex;size:64;not null" json:"ip_address"`
	FailedAttempts int        `gorm:"default:0;not null" json:"failed_attempts"`
	LockoutUntil  *time.Time `json:"lockout_until"`
	LastAttemptAt *time.Time `json:"last_attempt_at"`
}

type UserMusic struct {
	BaseModel
	UserID   uint   `gorm:"not null" json:"user_id"`
	Title    string `gorm:"size:200;not null" json:"title"`
	Artist   string `gorm:"size:200" json:"artist"`
	FileURL  string `gorm:"size:500;not null" json:"file_url"`
	CoverURL string `gorm:"size:500" json:"cover_url"`
	AddedAt  time.Time `json:"added_at"`
	
	User User `gorm:"foreignKey:UserID" json:"-"`
}

type Friendship struct {
	BaseModel
	UserLowID  uint `gorm:"not null;index" json:"user_low_id"`
	UserHighID uint `gorm:"not null;index" json:"user_high_id"`
	CreatedAt  time.Time `json:"created_at"`
	
	UserLow  User `gorm:"foreignKey:UserLowID" json:"-"`
	UserHigh User `gorm:"foreignKey:UserHighID" json:"-"`
}

type FriendRequest struct {
	BaseModel
	FromUserID  uint   `gorm:"not null;index" json:"from_user_id"`
	ToUserID    uint   `gorm:"not null;index" json:"to_user_id"`
	Status      string `gorm:"size:20;default:'pending'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	RespondedAt *time.Time `json:"responded_at"`
	
	FromUser User `gorm:"foreignKey:FromUserID" json:"-"`
	ToUser   User `gorm:"foreignKey:ToUserID" json:"-"`
}

// ============================================================================
// Chat models
// ============================================================================

type Room struct {
	BaseModel
	Name        string `gorm:"size:150" json:"name"`
	Type        string `gorm:"size:20;not null" json:"type"` // 'dm', 'server', 'broadcast'
	IsPublic    bool   `gorm:"default:false" json:"is_public"`
	OwnerID     *uint  `gorm:"index" json:"owner_id"`
	Description string `gorm:"size:500" json:"description"`
	AvatarURL   string `gorm:"size:300" json:"avatar_url"`
	BannerURL   string `gorm:"size:300" json:"banner_url"`
	InviteToken string `gorm:"size:100;unique" json:"invite_token"`
	LinkedChatID *uint `json:"linked_chat_id"`

	Owner    User      `gorm:"foreignKey:OwnerID" json:"owner"`
	Channels []Channel `gorm:"foreignKey:RoomID" json:"channels"`
	Members  []Member  `gorm:"foreignKey:RoomID" json:"members"`
	Roles    []Role    `gorm:"foreignKey:RoomID" json:"roles"`
	Bans     []RoomBan `gorm:"foreignKey:RoomID" json:"-"`
}

type Channel struct {
	BaseModel
	Name               string `gorm:"size:100;not null" json:"name"`
	RoomID             uint   `gorm:"not null;index" json:"room_id"`
	Description        string `gorm:"size:500" json:"description"`
	IconEmoji          string `gorm:"size:10" json:"icon_emoji"`
	IconImageURL       string `gorm:"size:300" json:"icon_image_url"`
	WriterRoleIDsJSON  string `gorm:"type:text" json:"writer_role_ids_json"`

	Room     Room      `gorm:"foreignKey:RoomID" json:"room"`
	Messages []Message `gorm:"foreignKey:ChannelID" json:"messages"`
	ReadBy   []ReadMessage `gorm:"foreignKey:ChannelID" json:"-"`
}

type Member struct {
	BaseModel
	UserID     uint   `gorm:"not null;uniqueIndex:idx_user_room" json:"user_id"`
	RoomID     uint   `gorm:"not null;uniqueIndex:idx_user_room" json:"room_id"`
	Role       string `gorm:"size:20;default:'member'" json:"role"`
	MutedUntil *time.Time `json:"muted_until"`
	
	User User `gorm:"foreignKey:UserID" json:"user"`
	Room Room `gorm:"foreignKey:RoomID" json:"room"`
}

type Role struct {
	BaseModel
	RoomID                   uint   `gorm:"not null;index" json:"room_id"`
	Name                     string `gorm:"size:60;not null" json:"name"`
	MentionTag               string `gorm:"size:60;not null" json:"mention_tag"`
	IsSystem                 bool   `gorm:"default:false" json:"is_system"`
	CanBeMentionedByEveryone bool   `gorm:"default:false" json:"can_be_mentioned_by_everyone"`
	PermissionsJSON          string `gorm:"type:text" json:"permissions_json"`
	CreatedAt                time.Time `json:"created_at"`
	
	Room       Room        `gorm:"foreignKey:RoomID" json:"room"`
	MemberLinks []MemberRole `gorm:"foreignKey:RoleID" json:"-"`
	CanMention []RoleMentionPermission `gorm:"foreignKey:SourceRoleID" json:"-"`
	MentionableBy []RoleMentionPermission `gorm:"foreignKey:TargetRoleID" json:"-"`
}

type MemberRole struct {
	BaseModel
	UserID  uint `gorm:"not null;index" json:"user_id"`
	RoomID  uint `gorm:"not null;index" json:"room_id"`
	RoleID  uint `gorm:"not null;index" json:"role_id"`
	AssignedAt time.Time `json:"assigned_at"`
	
	User User `gorm:"foreignKey:UserID" json:"user"`
	Room Room `gorm:"foreignKey:RoomID" json:"room"`
	Role Role `gorm:"foreignKey:RoleID" json:"role"`
}

type RoleMentionPermission struct {
	BaseModel
	RoomID       uint `gorm:"not null;index" json:"room_id"`
	SourceRoleID uint `gorm:"not null;index" json:"source_role_id"`
	TargetRoleID uint `gorm:"not null;index" json:"target_role_id"`
	CreatedAt    time.Time `json:"created_at"`
	
	SourceRole Role `gorm:"foreignKey:SourceRoleID" json:"source_role"`
	TargetRole Role `gorm:"foreignKey:TargetRoleID" json:"target_role"`
}

type RoomBan struct {
	BaseModel
	RoomID      uint   `gorm:"not null;index" json:"room_id"`
	UserID      uint   `gorm:"not null;index" json:"user_id"`
	BannedByID  *uint  `gorm:"index" json:"banned_by_id"`
	Reason      string `gorm:"size:500" json:"reason"`
	BannedAt    time.Time `json:"banned_at"`
	BannedUntil *time.Time `json:"banned_until"`
	MessagesDeleted bool `gorm:"default:false" json:"messages_deleted"`

	Room     Room `gorm:"foreignKey:RoomID" json:"room"`
	User     User `gorm:"foreignKey:UserID" json:"user"`
	BannedBy User `gorm:"foreignKey:BannedByID" json:"banned_by"`
}

// ============================================================================
// Content models
// ============================================================================

type Message struct {
	BaseModel
	Content     string     `gorm:"type:text;not null" json:"content"`
	Timestamp   time.Time  `json:"timestamp"`
	EditedAt    *time.Time `json:"edited_at"`
	UserID      uint       `gorm:"not null;index" json:"user_id"`
	ChannelID   uint       `gorm:"not null;index" json:"channel_id"`
	MessageType string     `gorm:"size:20;default:'text'" json:"message_type"`
	FileURL     string     `gorm:"size:500" json:"file_url"`
	FileName    string     `gorm:"size:200" json:"file_name"`
	FileSize    int64      `json:"file_size"`
	ReplyToID   *uint      `json:"reply_to_id"`

	User    User     `gorm:"foreignKey:UserID" json:"user"`
	Channel Channel  `gorm:"foreignKey:ChannelID" json:"channel"`
	Reactions []MessageReaction `gorm:"foreignKey:MessageID" json:"reactions"`
	ReplyTo *Message `gorm:"foreignKey:ReplyToID" json:"reply_to"`
}

type MessageReaction struct {
	BaseModel
	MessageID    uint   `gorm:"not null;index" json:"message_id"`
	UserID       uint   `gorm:"not null;index" json:"user_id"`
	Emoji        string `gorm:"size:50;not null" json:"emoji"`
	ReactionType string `gorm:"size:20;default:'emoji'" json:"reaction_type"`

	Message Message `gorm:"foreignKey:MessageID" json:"message"`
	User    User    `gorm:"foreignKey:UserID" json:"user"`
}

type ReadMessage struct {
	BaseModel
	UserID            uint       `gorm:"not null;index" json:"user_id"`
	ChannelID         uint       `gorm:"not null;index" json:"channel_id"`
	LastReadMessageID *uint      `json:"last_read_message_id"`
	LastReadAt        time.Time  `json:"last_read_at"`

	User    User    `gorm:"foreignKey:UserID" json:"user"`
	Channel Channel `gorm:"foreignKey:ChannelID" json:"channel"`
}

type StickerPack struct {
	BaseModel
	Name      string `gorm:"size:100;not null" json:"name"`
	IconEmoji string `gorm:"size:10" json:"icon_emoji"`
	OwnerID   uint   `gorm:"not null;index" json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	
	Owner    User      `gorm:"foreignKey:OwnerID" json:"owner"`
	Stickers []Sticker `gorm:"foreignKey:PackID" json:"stickers"`
}

type Sticker struct {
	BaseModel
	Name     string `gorm:"size:100;not null" json:"name"`
	FileURL  string `gorm:"size:500;not null" json:"file_url"`
	PackID   uint   `gorm:"not null;index" json:"pack_id"`
	OwnerID  uint   `gorm:"not null;index" json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`

	Pack  StickerPack `gorm:"foreignKey:PackID" json:"pack"`
	Owner User        `gorm:"foreignKey:OwnerID" json:"owner"`
}
