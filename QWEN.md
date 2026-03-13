# BoxChat Go Backend

## Project Overview

BoxChat is a self-hosted messenger application with real-time messaging capabilities. The backend is written in Go with a React-based frontend.

### Tech Stack

| Layer | Technologies |
|-------|--------------|
| **Backend** | Go 1.21+, Gin, GORM, gorilla/websocket (Socket.IO v4 manual implementation) |
| **Frontend** | Vite, React 19, TypeScript, Material UI (MUI), socket.io-client |
| **Database** | SQLite (via GORM ORM) |
| **Real-time** | Socket.IO v4 (manual implementation on gorilla/websocket) |

### Core Features

- User accounts with authentication (login/register/session management)
- Multi-channel rooms with role-based permissions
- Direct messages (DMs) and friend requests
- Real-time messaging via Socket.IO
- Message actions: edit, delete, forward, reactions, replies
- Mentions and role-based mentions (@everyone, @role, @username)
- Media uploads: images, audio, video, files
- GIF integration (Giphy)
- Presence indicators and read receipts
- Moderation: mute, kick, ban (temporary)
- File uploads with organized subdirectories
- Music library and sticker packs

## Directory Structure

```
BoxChat/
├── go/                       # Go backend
│   ├── cmd/
│   │   └── server/
│   │       └── main.go       # Application entry point
│   ├── internal/
│   │   ├── config/           # Configuration loading
│   │   ├── database/         # Database initialization, migrations
│   │   ├── models/           # GORM models
│   │   ├── handlers/
│   │   │   ├── http/         # HTTP handlers (REST API)
│   │   │   │   ├── auth.go   # Authentication endpoints
│   │   │   │   ├── api.go    # Main REST API
│   │   │   │   ├── friends.go
│   │   │   │   ├── search.go
│   │   │   │   ├── rooms.go
│   │   │   │   ├── channels.go
│   │   │   │   ├── roles.go
│   │   │   │   ├── admin.go
│   │   │   │   ├── music.go
│   │   │   │   ├── stickers.go
│   │   │   │   └── upload.go
│   │   │   └── socketio/     # Socket.IO v4 handlers
│   │   ├── middleware/       # Auth, CORS, Logger
│   │   ├── services/         # Business logic
│   │   └── utils/            # Helper utilities
│   ├── go.mod
│   └── README.md
├── frontend/                 # React SPA
│   ├── src/
│   │   ├── main.tsx         # Entry point
│   │   ├── router.tsx       # React Router configuration
│   │   ├── ui/              # Reusable UI components
│   │   ├── views/           # Page-level components
│   │   ├── utils/           # Utility functions
│   │   └── assets/          # Static assets
│   ├── package.json
│   └── vite.config.ts
├── config.json               # JSON configuration file
├── uploads/                  # User-uploaded files (git-ignored)
│   ├── avatars/
│   ├── room_avatars/
│   ├── channel_icons/
│   ├── files/
│   ├── music/
│   └── videos/
└── boxchat-go                # Compiled Go binary
```

## Building and Running

### Prerequisites

- Go 1.21 or higher
- Node.js and npm (for frontend)

### Backend Build

```bash
cd go
go mod tidy
go build -o ../boxchat-go ./cmd/server
```

### Backend Run

```bash
# From project root with environment variables
ADMIN_PASSWORD="YourPassword123!" \
SECRET_KEY="$(openssl rand -hex 32)" \
ALLOWED_ORIGINS="http://localhost,http://127.0.0.1" \
./boxchat-go

# Or from go directory
cd go
ADMIN_PASSWORD="YourPassword123!" \
SECRET_KEY="$(openssl rand -hex 32)" \
ALLOWED_ORIGINS="http://localhost,http://127.0.0.1" \
go run ./cmd/server
```

### Frontend Development

```bash
cd frontend
npm run dev      # Start Vite dev server (hot reload)
npm run build    # Build production assets to dist/
npm run lint     # Run ESLint
npm run preview  # Preview production build
```

### Server Configuration

Configuration is loaded from `config.json` in the project root.

Environment variables:
- `ADMIN_PASSWORD` - Initial admin password
- `SECRET_KEY` - Session security key
- `ALLOWED_ORIGINS` - CORS allowed origins (comma-separated)
- `SQLALCHEMY_DATABASE_URI` - Database connection string
- `GIPHY_API_KEY` - Giphy API key
- `SERVER_HOST` - Server host (default: 127.0.0.1)
- `SERVER_PORT` - Server port (default: 5000)

## API Endpoints

### Authentication
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/register` - User registration
- `GET /api/v1/auth/session` - Session validation
- `GET /logout` - Logout

### User
- `GET /api/v1/user/me` - Current user
- `PATCH /api/v1/user/settings` - Update settings
- `POST /api/v1/user/avatar` - Upload avatar
- `DELETE /api/v1/user/avatar` - Remove avatar
- `POST /api/v1/user/delete` - Delete account

### Friends
- `GET /api/v1/friends/status/:user_id` - Check friendship status
- `POST /api/v1/friends/request` - Send friend request
- `GET /api/v1/friends/requests` - List friend requests
- `POST /api/v1/friends/requests/:id/respond` - Accept/decline request
- `DELETE /api/v1/friends/requests/:id` - Cancel request
- `GET /api/v1/friends` - List friends
- `DELETE /api/v1/friends/:id` - Remove friend
- `POST /api/v1/dm/:user_id/create` - Create DM room

### Search
- `GET /api/v1/search/users?q=query&limit=20` - Search users
- `GET /api/v1/search/servers?q=query&limit=20` - Search servers
- `GET /api/v1/search?q=query&limit=10` - Global search (users, rooms, messages)

### Rooms & Channels
- `GET /api/v1/rooms` - List user's rooms
- `GET /api/v1/room/:room_id` - Room info
- `POST /api/v1/room/:room_id/join` - Join room
- `GET /api/v1/room/:room_id/members` - Room members
- `GET /api/v1/room/:room_id/roles` - Room roles
- `GET /api/v1/channel/:channel_id/messages` - Channel messages
- `POST /api/v1/channel/:channel_id/mark_read` - Mark as read
- `GET /channels/accessible` - User's accessible channels

### Channel Management
- `POST /api/v1/room/:room_id/add_channel` - Create channel
- `PATCH /api/v1/channel/:channel_id/edit` - Edit channel
- `DELETE /api/v1/channel/:channel_id/delete` - Delete channel
- `PATCH /api/v1/channel/:channel_id/permissions` - Update write permissions

### Room Management
- `GET /api/v1/room/:room_id/settings` - Room settings
- `PATCH /api/v1/room/:room_id/settings` - Update room settings
- `POST /api/v1/room/:room_id/avatar/delete` - Remove room avatar
- `DELETE /api/v1/room/:room_id/delete` - Delete room
- `GET /api/v1/room/:room_id/bans` - Room bans
- `POST /api/v1/room/:room_id/unban/:user_id` - Unban user

### Role Management
- `POST /api/v1/room/:room_id/roles` - Create role
- `GET /api/v1/room/:room_id/roles/:role_id` - Role info
- `PATCH /api/v1/room/:room_id/roles/:role_id` - Update role
- `DELETE /api/v1/room/:room_id/roles/:role_id` - Delete role
- `PATCH /api/v1/room/:room_id/roles/:role_id/permissions` - Update role permissions
- `POST /api/v1/room/:room_id/roles/mention_permissions` - Add mention permission
- `DELETE /api/v1/room/:room_id/roles/mention_permissions` - Remove mention permission
- `POST /api/v1/room/:room_id/members/:member_user_id/roles` - Assign role to member
- `DELETE /api/v1/room/:room_id/members/:member_user_id/roles/:role_id` - Remove role from member

### Messages
- `POST /api/v1/message/:message_id/reaction` - Add reaction
- `POST /api/v1/message/:message_id/delete` - Delete message
- `POST /api/v1/message/:message_id/edit` - Edit message
- `POST /api/v1/message/:message_id/forward` - Forward message

### Music Library
- `POST /api/v1/music/add` - Add track
- `GET /api/v1/user/music` - List tracks
- `POST /api/v1/music/:music_id/delete` - Delete track

### Sticker Packs
- `POST /api/v1/sticker_packs` - Create sticker pack
- `GET /api/v1/sticker_packs` - List sticker packs
- `GET /api/v1/sticker_packs/:pack_id` - Sticker pack info
- `PATCH /api/v1/sticker_packs/:pack_id` - Update sticker pack
- `DELETE /api/v1/sticker_packs/:pack_id` - Delete sticker pack
- `POST /api/v1/sticker_packs/:pack_id/stickers` - Add sticker
- `DELETE /api/v1/stickers/:sticker_id` - Delete sticker

### Administration
- `POST /api/v1/admin/user/:user_id/ban` - Ban (globally or in room)
- `POST /api/v1/admin/user/:user_id/unban` - Unban
- `GET /api/v1/admin/banned_users` - List banned users
- `GET /api/v1/admin/banned_ips` - List banned IPs
- `POST /api/v1/admin/user/:user_id/kick_from_room/:room_id` - Kick
- `POST /api/v1/admin/user/:user_id/mute_in_room/:room_id` - Mute
- `POST /api/v1/admin/user/:user_id/unmute_in_room/:room_id` - Unmute
- `POST /api/v1/admin/user/:user_id/promote` - Promote
- `POST /api/v1/admin/user/:user_id/demote` - Demote
- `POST /api/v1/admin/user/:user_id/delete_messages` - Delete messages
- `POST /api/v1/admin/user/:user_id/change_password` - Change user password
- `POST /api/v1/user/change_password` - Change own password

### Additional
- `POST /api/v1/room/:room_id/invite` - Create invite
- `GET /api/v1/join/:token` - Join by invite
- `POST /api/v1/room/:room_id/leave` - Leave room
- `POST /api/v1/room/:room_id/delete_dm` - Delete DM
- `GET /api/v1/user/:user_id/profile` - User profile
- `GET /api/v1/statistics` - Server statistics (superuser)
- `POST /api/v1/room/:room_id/banner` - Room banner
- `DELETE /api/v1/room/:room_id/banner/delete` - Remove banner

### GIF (Giphy)
- `GET /api/v1/gifs/trending?limit=24&offset=0` - Trending GIFs
- `GET /api/v1/gifs/search?q=query&limit=24&offset=0` - Search GIFs

### File Upload
- `POST /upload_file` - Upload file
- `GET /uploads/:filepath` - Get file

### WebSocket (Socket.IO v4)
- `GET /socket.io` - Socket.IO connection (requires authentication)

## Socket.IO Events

### Client → Server

| Event | Payload | Description |
|-------|---------|-------------|
| `join` | `{ channel_id }` | Join channel room |
| `send_message` | `{ room_id, channel_id, msg, message_type }` | Send message |
| `read` | `{ channel_id }` | Mark channel as read |

### Server → Client

| Event | Payload | Description |
|-------|---------|-------------|
| `connect` | - | Connection established |
| `receive_message` | Message object | New message in channel |
| `message_deleted` | `{ message_id }` | Message was deleted |
| `message_edited` | Edited message | Message was edited |
| `presence_updated` | `{ user_id, status }` | User presence changed |
| `member_mute_updated` | Mute data | User was muted |
| `member_removed` | Removal data | User removed from room |
| `force_redirect` | Redirect data | Redirect after kick/ban |
| `command_result` | Result data | Moderation command result |
| `read_status_updated` | Read data | Read status changed |
| `new_dm_created` | DM room data | New DM room created |
| `new_dm_message` | Message data | New DM message |
| `server_removed` | Removal data | User removed from server |
| `bulk_messages_deleted` | Deletion data | Bulk message deletion |
| `room_state_refresh` | Room state | Refresh room state |
| `friend_request_updated` | Request data | Friend request status changed |
| `error` | Error data | Error occurred |

## Moderation Commands

Commands are sent via WebSocket as messages starting with `/`:

| Command | Syntax | Description |
|---------|--------|-------------|
| `/mute` | `/mute @username 30m [reason]` | Mute user for duration |
| `/unmute` | `/unmute @username` | Unmute user |
| `/kick` | `/kick @username [reason]` | Kick user from room |
| `/ban` | `/ban @username [duration] [reason]` | Ban user (optionally temporary) |

**Required Permissions:**
- `mute_members` - for /mute and /unmute
- `kick_members` - for /kick
- `ban_members` - for /ban
- Superuser and room admin have all permissions

## Mention System

Automatic parsing of mentions in messages:

- `@username` - User mention
- `@role` - Role mention (if permitted)
- `@everyone` - Mention everyone in room (if permitted)

Server returns `mentions` structure in `receive_message` event:
```json
{
  "mention_everyone": false,
  "mentioned_user_ids": [1, 2, 3],
  "mentioned_usernames": ["user1", "user2"],
  "mentioned_role_ids": [1],
  "mentioned_role_tags": ["admin"],
  "denied_role_tags": []
}
```

## Database Models

### User Models
- `User` - User account with profile, settings, and ban management
- `AuthThrottle` - Login attempt throttling
- `Friendship` - Friend relationships
- `FriendRequest` - Pending friend requests
- `UserMusic` - User's music library

### Chat Models
- `Room` - Chat rooms (DM, server, broadcast)
- `Channel` - Channels within rooms
- `Member` - Room membership
- `Role` - Room roles with permissions
- `MemberRole` - Role assignments
- `RoleMentionPermission` - Role mention permissions
- `RoomBan` - Room-level bans

### Content Models
- `Message` - Chat messages
- `MessageReaction` - Message reactions
- `ReadMessage` - Read status tracking
- `StickerPack` - Sticker packs
- `Sticker` - Individual stickers

## Development Conventions

### Backend (Go)

- **Project Layout:** Standard Go layout with `cmd/` and `internal/`
- **Handlers:** HTTP handlers in `internal/handlers/http/`, WebSocket in `internal/handlers/socketio/`
- **Services:** Business logic in `internal/services/`
- **Models:** GORM models in `internal/models/`
- **Middleware:** Auth, CORS, logging in `internal/middleware/`

### Frontend (React/TypeScript)

- **Component Structure:**
  - `frontend/src/ui/` - Shared/reusable components
  - `frontend/src/views/` - Page-level components
- **Styling:** Material UI (MUI) with Emotion
- **State:** Local state + Socket.IO for real-time updates
- **Routing:** React Router v7 with loader-based auth
- **Virtualization:** `react-virtuoso` for message lists

### Code Style

- **Go:** Standard Go conventions (gofmt, go vet)
- **TypeScript:** Strict typing with TypeScript 5.9+
- **Linting:** ESLint configured for React/TypeScript

## Key Implementation Details

### Data Flow (Sending a Message)

1. Client emits `send_message` via Socket.IO with payload
2. Server validates membership, permissions, bans, and file URLs
3. Message created in database via GORM
4. Server emits `receive_message` to channel room
5. Server emits `message_notification` to per-user notification rooms

### Permissions & Roles

- Rooms have roles (`Role` model) linked via `MemberRole`
- Permission keys: `manage_server`, `manage_roles`, `manage_channels`, `invite_members`, `delete_server`, `delete_messages`, `kick_members`, `ban_members`, `mute_members`
- Mention permissions tracked in `RoleMentionPermission`

### File Uploads

- Files stored in `uploads/` with organized subdirectories
- Server validates local files and whitelisted external hosts (e.g., giphy.com)
- Allowed extensions configured in `config.json`

### Security

- Cookie-based session authentication
- bcrypt password hashing
- Login throttling and account lockout
- CORS middleware
- Authentication middleware for protected routes

## Common Tasks

### Add a New API Endpoint

1. Add handler in `internal/handlers/http/`
2. Register route in handler's `RegisterRoutes` method
3. Update API documentation

### Add a New Socket Event

1. Add event handler in `internal/handlers/socketio/`
2. Emit event to appropriate rooms
3. Update frontend to listen/emit

### Add a New Frontend Page

1. Create component in `frontend/src/views/`
2. Add route in `frontend/src/router.tsx`
3. Use auth loader for protected routes

### Database Schema Changes

1. Update model in `internal/models/models.go`
2. Add migration logic in `internal/database/`
3. Run migrations on startup

## Related Files

- `go/README.md` - Go backend documentation
- `config.json` - Application configuration
- `frontend/src/router.tsx` - Frontend routing
- `go/internal/models/models.go` - GORM models
- `go/internal/handlers/socketio/` - WebSocket handlers
