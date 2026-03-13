// Package socketio implements a Socket.IO v4 compatible server using gorilla/websocket
package socketio

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// ============================================================================
// Socket.IO Protocol Constants
// ============================================================================

// Engine.IO protocol version
const (
	EIO_VERSION = "4"
)

// Socket.IO packet types
const (
	PACKET_CONNECT      = iota // 0
	PACKET_DISCONNECT          // 1
	PACKET_EVENT               // 2
	PACKET_ACK                 // 3
	PACKET_CONNECT_ERROR       // 4
	PACKET_BINARY_EVENT        // 5
	PACKET_BINARY_ACK          // 6
)

// Engine.IO packet types
const (
	EIO_OPEN    = iota // 0 - Open
	EIO_CLOSE          // 1 - Close
	EIO_PING           // 2 - Ping
	EIO_PONG           // 3 - Pong
	EIO_MESSAGE        // 4 - Message
	EIO_UPGRADE        // 5 - Upgrade
	EIO_NOOP           // 6 - Noop
)

// ============================================================================
// Configuration
// ============================================================================

// getAllowedOrigins returns list of allowed origins from environment variable
func getAllowedOrigins() []string {
	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")
	if allowedOriginsEnv == "" {
		return []string{"http://localhost", "http://127.0.0.1", "http://localhost:5000", "http://127.0.0.1:5000"}
	}
	return strings.Split(allowedOriginsEnv, ",")
}

// checkOrigin validates that the origin is allowed
func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // Allow if no origin header (e.g., mobile apps)
	}

	allowedOrigins := getAllowedOrigins()
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if strings.HasPrefix(origin, allowed) {
			return true
		}
	}

	log.Printf("[SOCKET.IO] Blocked connection from origin: %s", origin)
	return false
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     checkOrigin,
}

// ============================================================================
// Client and Hub
// ============================================================================

// Client represents a Socket.IO client connection
type Client struct {
	ID        string // Socket.IO session ID
	User      *models.User
	Conn      *websocket.Conn
	Send      chan []byte
	Rooms     map[string]bool // Rooms/channels the client is subscribed to
	mu        sync.RWMutex
	pingTimer *time.Timer
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	rooms      map[string]map[*Client]bool // room name -> set of clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *broadcastMessage
	mu         sync.RWMutex
}

type broadcastMessage struct {
	room string
	data []byte
}

var hub = &Hub{
	clients:    make(map[*Client]bool),
	rooms:      make(map[string]map[*Client]bool),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	broadcast:  make(chan *broadcastMessage, 256),
}

// generateSID generates a random Socket.IO session ID
func generateSID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:20]
}

// ============================================================================
// Hub Management
// ============================================================================

// InitHub initializes the Socket.IO hub
func InitHub() {
	go func() {
		for {
			select {
			case client := <-hub.register:
				hub.mu.Lock()
				hub.clients[client] = true
				hub.mu.Unlock()
				log.Printf("[SOCKET.IO] Client connected: %s (user %d)", client.ID, client.User.ID)

			case client := <-hub.unregister:
				hub.mu.Lock()
				if _, ok := hub.clients[client]; ok {
					delete(hub.clients, client)
					// Remove from all rooms
					for room := range client.Rooms {
						if roomClients, ok := hub.rooms[room]; ok {
							delete(roomClients, client)
							if len(roomClients) == 0 {
								delete(hub.rooms, room)
							}
						}
					}
					close(client.Send)
				}
				hub.mu.Unlock()
				log.Printf("[SOCKET.IO] Client disconnected: %s (user %d)", client.ID, client.User.ID)

			case msg := <-hub.broadcast:
				hub.mu.RLock()
				if roomClients, ok := hub.rooms[msg.room]; ok {
					for client := range roomClients {
						select {
						case client.Send <- msg.data:
						default:
							// Client buffer full, mark for cleanup
							select {
							case hub.unregister <- client:
							default:
							}
						}
					}
				}
				hub.mu.RUnlock()
			}
		}
	}()
}

// joinRoom adds a client to a room
func (h *Hub) joinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true
	client.Rooms[room] = true
}

// leaveRoom removes a client from a room
func (h *Hub) leaveRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if roomClients, ok := h.rooms[room]; ok {
		delete(roomClients, client)
		delete(client.Rooms, room)
		if len(roomClients) == 0 {
			delete(h.rooms, room)
		}
	}
}

// broadcastToRoom sends a message to all clients in a room
func (h *Hub) broadcastToRoom(room string, data []byte) {
	h.broadcast <- &broadcastMessage{room: room, data: data}
}

// ============================================================================
// Socket.IO Protocol Encoding/Decoding
// ============================================================================

// encodeEngineIO encodes an Engine.IO packet
func encodeEngineIO(packetType int, data string) string {
	return fmt.Sprintf("%d%s", packetType, data)
}

// encodeSocketIO encodes a Socket.IO event
func encodeSocketIO(eventType int, event string, data interface{}) string {
	jsonData, _ := json.Marshal(data)
	return fmt.Sprintf("%d[\"%s\",%s]", eventType, event, string(jsonData))
}

// encodeSocketIOWithNamespace encodes a Socket.IO event with namespace
func encodeSocketIOWithNamespace(eventType int, namespace string, event string, data interface{}) string {
	jsonData, _ := json.Marshal(data)
	if namespace == "/" || namespace == "" {
		return fmt.Sprintf("%d[\"%s\",%s]", eventType, event, string(jsonData))
	}
	return fmt.Sprintf("%d%s,[\"%s\",%s]", eventType, namespace, event, string(jsonData))
}

// decodeEngineIO decodes an Engine.IO packet type
func decodeEngineIO(message string) (int, string, error) {
	if len(message) == 0 {
		return -1, "", fmt.Errorf("empty message")
	}
	packetType := int(message[0] - '0')
	return packetType, message[1:], nil
}

// ============================================================================
// WebSocket Handler
// ============================================================================

// WSHandler handles WebSocket connections with Socket.IO protocol
func WSHandler(c *gin.Context) {
	// Get user from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get user from database
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[SOCKET.IO] Upgrade error: %v", err)
		return
	}

	// Generate session ID
	sid := generateSID()

	// Create client
	client := &Client{
		ID:    sid,
		User:  &user,
		Conn:  conn,
		Send:  make(chan []byte, 256),
		Rooms: make(map[string]bool),
	}

	// Send Engine.IO open packet
	openPacket := map[string]interface{}{
		"sid":          sid,
		"upgrades":     []string{"websocket"},
		"pingInterval": 25000,
		"pingTimeout":  5000,
	}
	openJSON, _ := json.Marshal(openPacket)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_OPEN, string(openJSON))))

	// Register client
	hub.register <- client

	// Update user presence
	user.PresenceStatus = "online"
	user.LastSeen = nil
	if err := database.DB.Save(&user).Error; err != nil {
		log.Printf("[SOCKET.IO] Failed to update user presence: %v", err)
	}

	// Emit presence update
	emitPresenceUpdate(&user)

	// Start read/write pumps
	go writePump(client)
	go readPump(client)
}

func writePump(client *Client) {
	defer func() {
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}

func readPump(client *Client) {
	defer func() {
		hub.unregister <- client

		// Update user presence
		client.User.PresenceStatus = "offline"
		now := time.Now()
		client.User.LastSeen = &now
		if err := database.DB.Save(client.User).Error; err != nil {
			log.Printf("[SOCKET.IO] Failed to update user presence: %v", err)
		}
		emitPresenceUpdate(client.User)

		client.Conn.Close()
	}()

	// Set read deadline for ping/pong
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(appData string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[SOCKET.IO] Read error: %v", err)
			}
			break
		}

		handleMessage(client, string(message))
	}
}

// handleMessage processes incoming Socket.IO messages
func handleMessage(client *Client, message string) {
	if len(message) == 0 {
		return
	}

	// Decode Engine.IO packet
	packetType, data, err := decodeEngineIO(message)
	if err != nil {
		log.Printf("[SOCKET.IO] Decode error: %v", err)
		return
	}

	switch packetType {
	case EIO_PING:
		// Respond with PONG
		client.Conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_PONG, data)))

	case EIO_MESSAGE:
		// Socket.IO message - check if it's a connection packet or event
		handleSocketIOMessage(client, data)

	case EIO_CLOSE:
		client.Conn.Close()
	}
}

// handleSocketIOMessage processes Socket.IO protocol messages
func handleSocketIOMessage(client *Client, data string) {
	if len(data) == 0 {
		return
	}

	// First character is the Socket.IO packet type
	packetType := int(data[0] - '0')
	payload := data[1:]

	// Handle namespace if present
	namespace := ""
	if len(payload) > 0 && payload[0] == '/' {
		// Find namespace end (comma or opening bracket)
		for i, ch := range payload {
			if ch == ',' || ch == '[' {
				namespace = payload[:i]
				payload = payload[i+1:]
				break
			}
		}
	}

	switch packetType {
	case PACKET_CONNECT:
		// Send connection acknowledgment
		response := encodeSocketIOWithNamespace(PACKET_CONNECT, namespace, "connect", map[string]interface{}{
			"sid": client.ID,
		})
		client.Conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))

	case PACKET_EVENT:
		// Parse event: ["event_name", {...}]
		handleEvent(client, namespace, payload)

	case PACKET_DISCONNECT:
		// Client disconnecting
		hub.unregister <- client
	}
}

// handleEvent processes Socket.IO events
func handleEvent(client *Client, namespace string, payload string) {
	// Parse JSON array: ["event_name", {...}]
	var eventArray []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &eventArray); err != nil {
		log.Printf("[SOCKET.IO] Event parse error: %v", err)
		return
	}

	if len(eventArray) < 1 {
		return
	}

	// Extract event name
	var eventName string
	if err := json.Unmarshal(eventArray[0], &eventName); err != nil {
		return
	}

	// Extract event data (if present)
	var eventData map[string]interface{}
	if len(eventArray) > 1 {
		if err := json.Unmarshal(eventArray[1], &eventData); err != nil {
			log.Printf("[SOCKET.IO] Event data parse error: %v", err)
			return
		}
	}

	log.Printf("[SOCKET.IO] Event: %s, Data: %v", eventName, eventData)

	// Route to appropriate handler
	switch eventName {
	case "join":
		handleJoin(client, eventData)
	case "send_message":
		handleSendMessage(client, eventData)
	case "read":
		handleRead(client, eventData)
	case "typing":
		// Handle typing indicator (not implemented yet)
	}
}

// ============================================================================
// Event Handlers
// ============================================================================

func handleJoin(client *Client, msg map[string]interface{}) {
	channelIDFloat, ok := msg["channel_id"].(float64)
	if !ok {
		return
	}

	channelID := fmt.Sprintf("%.0f", channelIDFloat)
	hub.joinRoom(client, channelID)

	log.Printf("[SOCKET.IO] User %d joined channel %s", client.User.ID, channelID)
}

func handleSendMessage(client *Client, msg map[string]interface{}) {
	channelIDFloat, ok := msg["channel_id"].(float64)
	if !ok {
		emitError(client.Conn, "Invalid channel_id")
		return
	}
	channelID := uint(channelIDFloat)

	content, _ := msg["msg"].(string)
	messageType, _ := msg["message_type"].(string)
	if messageType == "" {
		messageType = "text"
	}
	fileURL, _ := msg["file_url"].(string)
	fileName, _ := msg["file_name"].(string)
	fileSizeFloat, _ := msg["file_size"].(float64)
	fileSize := int64(fileSizeFloat)

	// Validate message content length
	const maxMessageLength = 10000
	if len(content) > maxMessageLength {
		emitError(client.Conn, "Message too long (max 10000 characters)")
		return
	}

	// Get channel and verify membership
	var channel models.Channel
	if err := database.DB.First(&channel, channelID).Error; err != nil {
		emitError(client.Conn, "Channel not found")
		return
	}

	// Verify user is member of the room
	var member models.Member
	if err := database.DB.Where("user_id = ? AND room_id = ?", client.User.ID, channel.RoomID).First(&member).Error; err != nil {
		emitError(client.Conn, "No access")
		return
	}

	// Check if muted
	if member.MutedUntil != nil && member.MutedUntil.After(time.Now()) {
		emitError(client.Conn, "You are muted")
		return
	}

	// Check for moderation commands
	if messageType == "text" && strings.HasPrefix(content, "/") {
		handleModerationCommand(client, channel.RoomID, content)
		return
	}

	// Parse mentions
	mentionService := services.NewMentionService()
	mentionData := mentionService.ParseMentions(content, channel.RoomID, client.User.ID)

	// Create message
	message := models.Message{
		Content:     content,
		UserID:      client.User.ID,
		ChannelID:   channelID,
		MessageType: messageType,
		FileURL:     fileURL,
		FileName:    fileName,
		FileSize:    fileSize,
		Timestamp:   time.Now(),
	}

	if err := database.DB.Create(&message).Error; err != nil {
		emitError(client.Conn, "Failed to save message")
		return
	}

	// Broadcast message
	broadcastMessageWithMentions(&message, client.User, channelID, mentionData)
}

func handleRead(client *Client, msg map[string]interface{}) {
	channelIDFloat, ok := msg["channel_id"].(float64)
	if !ok {
		return
	}
	channelID := uint(channelIDFloat)

	// Get last message in channel
	var lastMessage models.Message
	if err := database.DB.Where("channel_id = ?", channelID).
		Order("timestamp DESC").First(&lastMessage).Error; err != nil {
		return
	}

	// Update read status
	var readMsg models.ReadMessage
	result := database.DB.Where("user_id = ? AND channel_id = ?", client.User.ID, channelID).First(&readMsg)

	if result.Error == gorm.ErrRecordNotFound {
		readMsg = models.ReadMessage{
			UserID:            client.User.ID,
			ChannelID:         channelID,
			LastReadMessageID: &lastMessage.ID,
		}
		database.DB.Create(&readMsg)
	} else {
		database.DB.Model(&readMsg).Updates(map[string]interface{}{
			"last_read_message_id": lastMessage.ID,
		})
	}
}

// ============================================================================
// Message Broadcasting
// ============================================================================

// Message represents a chat message for broadcasting
type Message struct {
	ID           uint                `json:"id"`
	UserID       uint                `json:"user_id"`
	Username     string              `json:"username"`
	Avatar       string              `json:"avatar"`
	Content      string              `json:"msg"`
	TimestampISO string              `json:"timestamp_iso"`
	MessageType  string              `json:"message_type"`
	FileURL      string              `json:"file_url,omitempty"`
	FileName     string              `json:"file_name,omitempty"`
	FileSize     int64               `json:"file_size,omitempty"`
	EditedAtISO  *string             `json:"edited_at_iso,omitempty"`
	Reactions    map[string][]string `json:"reactions"`
	ReplyTo      interface{}         `json:"reply_to,omitempty"`
	Mentions     interface{}         `json:"mentions,omitempty"`
	RoomID       uint                `json:"room_id,omitempty"`
	ChannelID    uint                `json:"channel_id"`
}

func broadcastMessageWithMentions(message *models.Message, user *models.User, channelID uint, mentionData *services.MentionData) {
	msgData := Message{
		ID:           message.ID,
		UserID:       user.ID,
		Username:     user.Username,
		Avatar:       user.AvatarURL,
		Content:      message.Content,
		TimestampISO: message.Timestamp.Format(time.RFC3339),
		MessageType:  message.MessageType,
		FileURL:      message.FileURL,
		FileName:     message.FileName,
		FileSize:     message.FileSize,
		ChannelID:    channelID,
		Reactions:    make(map[string][]string),
		Mentions:     mentionData,
	}

	eventData := map[string]interface{}{
		"event":      "receive_message",
		"message":    msgData,
		"channel_id": channelID,
	}

	channelIDStr := fmt.Sprintf("%d", channelID)
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "receive_message", eventData)
	hub.broadcastToRoom(channelIDStr, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// ============================================================================
// Event Emitters
// ============================================================================

func emitPresenceUpdate(user *models.User) {
	eventData := map[string]interface{}{
		"event":         "presence_updated",
		"user_id":       user.ID,
		"username":      user.Username,
		"status":        user.PresenceStatus,
		"last_seen_iso": user.LastSeen,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "presence_updated", eventData)
	// Broadcast to all clients (they'll filter on their end)
	hub.mu.RLock()
	for client := range hub.clients {
		select {
		case client.Send <- []byte(encodeEngineIO(EIO_MESSAGE, response)):
		default:
		}
	}
	hub.mu.RUnlock()
}

func emitError(conn *websocket.Conn, message string) {
	eventData := map[string]interface{}{
		"event":   "error",
		"message": message,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "error", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitCommandResult(conn *websocket.Conn, ok bool, message string) {
	eventData := map[string]interface{}{
		"event":   "command_result",
		"ok":      ok,
		"message": message,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "command_result", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitMemberMuteUpdate(conn *websocket.Conn, roomID uint, update *services.MemberMuteUpdate) {
	eventData := map[string]interface{}{
		"event":       "member_mute_updated",
		"room_id":     roomID,
		"user_id":     update.UserID,
		"muted_until": update.MutedUntil,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "member_mute_updated", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitMemberRemoved(conn *websocket.Conn, removed *services.MemberRemoved) {
	eventData := map[string]interface{}{
		"event":   "member_removed",
		"user_id": removed.UserID,
		"room_id": removed.RoomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "member_removed", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitForceRedirect(conn *websocket.Conn, redirect *services.ForceRedirect) {
	eventData := map[string]interface{}{
		"event":    "force_redirect",
		"location": redirect.Location,
		"reason":   redirect.Reason,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "force_redirect", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitMessageDeleted(conn *websocket.Conn, messageID, channelID uint) {
	eventData := map[string]interface{}{
		"event":      "message_deleted",
		"message_id": messageID,
		"channel_id": channelID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "message_deleted", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitMessageEdited(conn *websocket.Conn, message *models.Message, editedAtISO string) {
	eventData := map[string]interface{}{
		"event":      "message_edited",
		"message_id": message.ID,
		"content":    message.Content,
		"channel_id": message.ChannelID,
		"edited_at":  editedAtISO,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "message_edited", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitNewDMCreated(conn *websocket.Conn, roomID uint, fromUserID uint, fromUsername, fromAvatar string) {
	eventData := map[string]interface{}{
		"event":         "new_dm_created",
		"room_id":       roomID,
		"from_user_id":  fromUserID,
		"from_username": fromUsername,
		"from_avatar":   fromAvatar,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "new_dm_created", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitReadStatusUpdated(conn *websocket.Conn, userID uint, username, channelID string) {
	eventData := map[string]interface{}{
		"event":      "read_status_updated",
		"user_id":    userID,
		"username":   username,
		"channel_id": channelID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "read_status_updated", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitNewDMMessage(conn *websocket.Conn, roomID uint) {
	eventData := map[string]interface{}{
		"event":   "new_dm_message",
		"room_id": roomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "new_dm_message", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitServerRemoved(conn *websocket.Conn, roomID uint) {
	eventData := map[string]interface{}{
		"event":   "server_removed",
		"room_id": roomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "server_removed", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitBulkMessagesDeleted(conn *websocket.Conn, userID, roomID uint, deleted int) {
	eventData := map[string]interface{}{
		"event":   "bulk_messages_deleted",
		"user_id": userID,
		"room_id": roomID,
		"deleted": deleted,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "bulk_messages_deleted", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitRoomStateRefresh(conn *websocket.Conn, roomID uint) {
	eventData := map[string]interface{}{
		"event":   "room_state_refresh",
		"room_id": roomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "room_state_refresh", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

func emitFriendRequestUpdated(conn *websocket.Conn, requestID uint, status string, byUserID uint, byUsername string, dmRoomID uint) {
	eventData := map[string]interface{}{
		"event":         "friend_request_updated",
		"request_id":    requestID,
		"status":        status,
		"by_user_id":    byUserID,
		"by_username":   byUsername,
		"dm_room_id":    dmRoomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "friend_request_updated", eventData)
	conn.WriteMessage(websocket.TextMessage, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// ============================================================================
// Global Broadcast Functions (for use by HTTP handlers)
// ============================================================================

// broadcastToUserRoom broadcasts to a user's notification room
func broadcastToUserRoom(userID uint, data []byte) {
	room := fmt.Sprintf("user_%d", userID)
	hub.broadcastToRoom(room, data)
}

// broadcastToChannel broadcasts to a channel room
func broadcastToChannel(channelID uint, data []byte) {
	room := fmt.Sprintf("%d", channelID)
	hub.broadcastToRoom(room, data)
}

// EmitMessageDeletedGlobal broadcasts message deleted event
func EmitMessageDeletedGlobal(messageID, channelID uint) {
	eventData := map[string]interface{}{
		"event":      "message_deleted",
		"message_id": messageID,
		"channel_id": channelID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "message_deleted", eventData)
	broadcastToChannel(channelID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitMessageEditedGlobal broadcasts message edited event
func EmitMessageEditedGlobal(message *models.Message, editedAtISO string) {
	eventData := map[string]interface{}{
		"event":      "message_edited",
		"message_id": message.ID,
		"content":    message.Content,
		"channel_id": message.ChannelID,
		"edited_at":  editedAtISO,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "message_edited", eventData)
	broadcastToChannel(message.ChannelID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitNewDMCreated sends notification when a new DM is created
func EmitNewDMCreated(userID, roomID uint, fromUsername, fromAvatar string) {
	eventData := map[string]interface{}{
		"event":         "new_dm_created",
		"room_id":       roomID,
		"from_username": fromUsername,
		"from_avatar":   fromAvatar,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "new_dm_created", eventData)
	broadcastToUserRoom(userID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitReadStatusUpdated broadcasts read status update
func EmitReadStatusUpdated(userID uint, username string, channelID uint) {
	eventData := map[string]interface{}{
		"event":      "read_status_updated",
		"user_id":    userID,
		"username":   username,
		"channel_id": channelID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "read_status_updated", eventData)
	broadcastToChannel(channelID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitNewDMMessage sends notification for new DM message
func EmitNewDMMessage(userID, roomID uint) {
	eventData := map[string]interface{}{
		"event":   "new_dm_message",
		"room_id": roomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "new_dm_message", eventData)
	broadcastToUserRoom(userID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitServerRemoved sends notification when removed from server
func EmitServerRemoved(userID, roomID uint) {
	eventData := map[string]interface{}{
		"event":   "server_removed",
		"room_id": roomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "server_removed", eventData)
	broadcastToUserRoom(userID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitBulkMessagesDeleted sends notification for bulk message deletion
func EmitBulkMessagesDeleted(userID, roomID uint, deleted int) {
	eventData := map[string]interface{}{
		"event":   "bulk_messages_deleted",
		"user_id": userID,
		"room_id": roomID,
		"deleted": deleted,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "bulk_messages_deleted", eventData)
	broadcastToUserRoom(userID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitRoomStateRefresh sends notification to refresh room state
func EmitRoomStateRefresh(userID, roomID uint) {
	eventData := map[string]interface{}{
		"event":   "room_state_refresh",
		"room_id": roomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "room_state_refresh", eventData)
	broadcastToUserRoom(userID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// EmitFriendRequestUpdated sends notification for friend request update
func EmitFriendRequestUpdated(userID, requestID uint, status string, byUserID uint, byUsername string, dmRoomID uint) {
	eventData := map[string]interface{}{
		"event":         "friend_request_updated",
		"request_id":    requestID,
		"status":        status,
		"by_user_id":    byUserID,
		"by_username":   byUsername,
		"dm_room_id":    dmRoomID,
	}
	response := encodeSocketIOWithNamespace(PACKET_EVENT, "", "friend_request_updated", eventData)
	broadcastToUserRoom(userID, []byte(encodeEngineIO(EIO_MESSAGE, response)))
}

// ============================================================================
// Moderation Commands
// ============================================================================

func handleModerationCommand(client *Client, roomID uint, content string) {
	modService := services.NewModerationService()

	parts := strings.Fields(content)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/mute":
		if len(parts) < 3 {
			emitCommandResult(client.Conn, false, "Usage: /mute @username <duration[m|h|d]> [reason]")
			return
		}
		username := parts[1]
		durationStr := parts[2]
		reason := ""
		if len(parts) > 3 {
			reason = strings.Join(parts[3:], " ")
		}

		result, update, err := modService.Mute(client.User.ID, roomID, username, durationStr, reason)
		if err != nil {
			emitCommandResult(client.Conn, result.OK, result.Message)
			return
		}

		emitMemberMuteUpdate(client.Conn, roomID, update)
		emitCommandResult(client.Conn, result.OK, result.Message)

	case "/unmute":
		if len(parts) < 2 {
			emitCommandResult(client.Conn, false, "Usage: /unmute @username")
			return
		}
		username := parts[1]

		result, update, err := modService.Unmute(client.User.ID, roomID, username)
		if err != nil {
			emitCommandResult(client.Conn, result.OK, result.Message)
			return
		}

		emitMemberMuteUpdate(client.Conn, roomID, update)
		emitCommandResult(client.Conn, result.OK, result.Message)

	case "/kick":
		if len(parts) < 2 {
			emitCommandResult(client.Conn, false, "Usage: /kick @username [reason]")
			return
		}
		username := parts[1]
		reason := ""
		if len(parts) > 2 {
			reason = strings.Join(parts[2:], " ")
		}

		result, removed, redirect, err := modService.Kick(client.User.ID, roomID, username, reason)
		if err != nil {
			emitCommandResult(client.Conn, result.OK, result.Message)
			return
		}

		emitMemberRemoved(client.Conn, removed)
		emitForceRedirect(client.Conn, redirect)
		emitCommandResult(client.Conn, result.OK, result.Message)

	case "/ban":
		if len(parts) < 2 {
			emitCommandResult(client.Conn, false, "Usage: /ban @username [duration] [reason]")
			return
		}
		username := parts[1]
		durationStr := ""
		reason := ""

		if len(parts) >= 3 {
			if matched, _ := regexp.MatchString(`^\d+[mhd]$`, parts[2]); matched {
				durationStr = parts[2]
				if len(parts) > 3 {
					reason = strings.Join(parts[3:], " ")
				}
			} else {
				reason = strings.Join(parts[2:], " ")
			}
		}

		result, removed, redirect, err := modService.Ban(client.User.ID, roomID, username, durationStr, reason)
		if err != nil {
			emitCommandResult(client.Conn, result.OK, result.Message)
			return
		}

		emitMemberRemoved(client.Conn, removed)
		emitForceRedirect(client.Conn, redirect)
		emitCommandResult(client.Conn, result.OK, result.Message)
	}
}
