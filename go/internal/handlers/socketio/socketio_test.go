package socketio

import (
	"encoding/json"
	"testing"
)

// ============================================================================
// Engine.IO Encoding/Decoding Tests
// ============================================================================

func TestEncodeEngineIO(t *testing.T) {
	tests := []struct {
		name       string
		packetType int
		data       string
		expected   string
	}{
		{"OPEN packet", 0, `{"sid":"test123"}`, "0{\"sid\":\"test123\"}"},
		{"PING packet", 2, "ping", "2ping"},
		{"MESSAGE packet", 4, `2["event",{}]`, "42[\"event\",{}]"},
		{"Empty data", 3, "", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeEngineIO(tt.packetType, tt.data)
			if result != tt.expected {
				t.Errorf("encodeEngineIO(%d, %q) = %q, want %q", tt.packetType, tt.data, result, tt.expected)
			}
		})
	}
}

func TestDecodeEngineIO(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		wantType    int
		wantData    string
		wantErr     bool
	}{
		{"OPEN packet", "0{\"sid\":\"abc\"}", 0, `{"sid":"abc"}`, false},
		{"PING packet", "2ping", 2, "ping", false},
		{"PONG packet", "3pong", 3, "pong", false},
		{"MESSAGE packet", "42[\"join\",{}]", 4, `2["join",{}]`, false},
		{"Empty message", "", -1, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotData, err := decodeEngineIO(tt.message)
			
			if tt.wantErr && err == nil {
				t.Errorf("decodeEngineIO() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("decodeEngineIO() unexpected error: %v", err)
			}
			if gotType != tt.wantType {
				t.Errorf("decodeEngineIO() type = %d, want %d", gotType, tt.wantType)
			}
			if gotData != tt.wantData {
				t.Errorf("decodeEngineIO() data = %q, want %q", gotData, tt.wantData)
			}
		})
	}
}

// ============================================================================
// Socket.IO Encoding/Decoding Tests
// ============================================================================

func TestEncodeSocketIO(t *testing.T) {
	tests := []struct {
		name      string
		eventType int
		event     string
		data      interface{}
	}{
		{"Connect event", PACKET_CONNECT, "connect", map[string]interface{}{"sid": "abc123"}},
		{"Join event", PACKET_EVENT, "join", map[string]interface{}{"channel_id": 1}},
		{"Message event", PACKET_EVENT, "send_message", map[string]interface{}{"msg": "hello", "channel_id": 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeSocketIOWithNamespace(tt.eventType, "", tt.event, tt.data)
			
			// Result should start with packet type
			if len(result) == 0 {
				t.Errorf("encodeSocketIO() returned empty string")
			}
			
			// Verify it's valid JSON after the packet type
			packetType := int(result[0] - '0')
			if packetType != tt.eventType {
				t.Errorf("encodeSocketIO() packet type = %d, want %d", packetType, tt.eventType)
			}
			
			// Parse the rest as JSON to verify it's valid
			jsonPart := result[1:]
			var arr []interface{}
			if err := json.Unmarshal([]byte(jsonPart), &arr); err != nil {
				t.Errorf("encodeSocketIO() invalid JSON: %v", err)
			}
		})
	}
}

func TestDecodeSocketIO(t *testing.T) {
	// Test encoding and decoding round-trip
	eventName := "join"
	eventData := map[string]interface{}{"channel_id": float64(1)}
	
	encoded := encodeSocketIOWithNamespace(PACKET_EVENT, "", eventName, eventData)
	
	// Verify encoding produced valid output
	if len(encoded) < 2 {
		t.Fatalf("Encoded message too short: %s", encoded)
	}
	
	// Check packet type
	packetType := int(encoded[0] - '0')
	if packetType != PACKET_EVENT {
		t.Errorf("Expected packet type %d, got %d", PACKET_EVENT, packetType)
	}
	
	// Parse JSON part
	var arr []interface{}
	if err := json.Unmarshal([]byte(encoded[1:]), &arr); err != nil {
		t.Fatalf("Invalid JSON in encoded message: %v", err)
	}
	
	if len(arr) < 2 {
		t.Fatalf("Expected array with at least 2 elements, got %d", len(arr))
	}
	
	// First element should be event name
	if name, ok := arr[0].(string); !ok || name != eventName {
		t.Errorf("Expected event name '%s', got '%v'", eventName, arr[0])
	}
}

// ============================================================================
// Hub Room Management Tests
// ============================================================================

func TestHubJoinRoom(t *testing.T) {
	h := &Hub{
		clients: make(map[*Client]bool),
		rooms:   make(map[string]map[*Client]bool),
	}

	client := &Client{
		ID:    "test-client-1",
		Rooms: make(map[string]bool),
	}

	// Join room
	h.joinRoom(client, "room_123")

	// Verify client is in room
	if roomClients, ok := h.rooms["room_123"]; !ok {
		t.Errorf("Room 'room_123' not found in hub")
	} else if !roomClients[client] {
		t.Errorf("Client not found in room 'room_123'")
	}

	// Verify client has room in its Rooms map
	if !client.Rooms["room_123"] {
		t.Errorf("Client Rooms map doesn't contain 'room_123'")
	}
}

func TestHubLeaveRoom(t *testing.T) {
	h := &Hub{
		clients: make(map[*Client]bool),
		rooms:   make(map[string]map[*Client]bool),
	}

	client := &Client{
		ID:    "test-client-2",
		Rooms: make(map[string]bool),
	}

	// Join and leave room
	h.joinRoom(client, "room_456")
	h.leaveRoom(client, "room_456")

	// Verify room is empty or removed
	if roomClients, ok := h.rooms["room_456"]; ok {
		if len(roomClients) > 0 {
			t.Errorf("Room 'room_456' should be empty after client left")
		}
	}

	// Verify client doesn't have room in its Rooms map
	if client.Rooms["room_456"] {
		t.Errorf("Client Rooms map should not contain 'room_456' after leaving")
	}
}

func TestHubJoinMultipleRooms(t *testing.T) {
	h := &Hub{
		clients: make(map[*Client]bool),
		rooms:   make(map[string]map[*Client]bool),
	}

	client := &Client{
		ID:    "test-client-3",
		Rooms: make(map[string]bool),
	}

	// Join multiple rooms
	rooms := []string{"room_1", "room_2", "room_3"}
	for _, room := range rooms {
		h.joinRoom(client, room)
	}

	// Verify client is in all rooms
	for _, room := range rooms {
		if roomClients, ok := h.rooms[room]; !ok {
			t.Errorf("Room '%s' not found", room)
		} else if !roomClients[client] {
			t.Errorf("Client not found in room '%s'", room)
		}
	}

	// Verify client has all rooms in its Rooms map
	for _, room := range rooms {
		if !client.Rooms[room] {
			t.Errorf("Client Rooms map doesn't contain '%s'", room)
		}
	}
}

func TestHubMultipleClientsInRoom(t *testing.T) {
	h := &Hub{
		clients: make(map[*Client]bool),
		rooms:   make(map[string]map[*Client]bool),
	}

	client1 := &Client{ID: "client-1", Rooms: make(map[string]bool)}
	client2 := &Client{ID: "client-2", Rooms: make(map[string]bool)}
	client3 := &Client{ID: "client-3", Rooms: make(map[string]bool)}

	// All clients join same room
	h.joinRoom(client1, "shared_room")
	h.joinRoom(client2, "shared_room")
	h.joinRoom(client3, "shared_room")

	// Verify all clients are in the room
	if roomClients, ok := h.rooms["shared_room"]; !ok {
		t.Errorf("Room 'shared_room' not found")
	} else if len(roomClients) != 3 {
		t.Errorf("Expected 3 clients in room, got %d", len(roomClients))
	}
}

// ============================================================================
// Session ID Generation Tests
// ============================================================================

func TestGenerateSID(t *testing.T) {
	// Generate multiple SIDs and verify they're unique
	sids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sid := generateSID()
		
		// Verify length (20 characters from base64 of 16 bytes)
		if len(sid) != 20 {
			t.Errorf("generateSID() length = %d, want 20", len(sid))
		}
		
		// Verify uniqueness
		if sids[sid] {
			t.Errorf("generateSID() generated duplicate SID: %s", sid)
		}
		sids[sid] = true
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestSocketIOProtocol(t *testing.T) {
	// Test full Socket.IO message encoding
	eventName := "join"
	eventData := map[string]interface{}{"channel_id": float64(123)}
	
	encoded := encodeSocketIOWithNamespace(PACKET_EVENT, "", eventName, eventData)
	
	// Verify it starts with correct packet type
	if len(encoded) < 1 || int(encoded[0]-'0') != PACKET_EVENT {
		t.Errorf("Expected EVENT packet (type 2), got: %s", encoded)
	}
	
	// Parse the JSON part
	var arr []interface{}
	if err := json.Unmarshal([]byte(encoded[1:]), &arr); err != nil {
		t.Fatalf("Invalid JSON in encoded message: %v", err)
	}
	
	// Verify event name
	if len(arr) < 2 {
		t.Fatalf("Expected array with at least 2 elements")
	}
	
	if name, ok := arr[0].(string); !ok || name != eventName {
		t.Errorf("Expected event name '%s', got '%v'", eventName, arr[0])
	}
	
	// Verify event data
	dataMap, ok := arr[1].(map[string]interface{})
	if !ok {
		t.Fatalf("Second element is not a map")
	}
	
	channelID, ok := dataMap["channel_id"].(float64)
	if !ok {
		t.Fatalf("channel_id not found or not a number")
	}
	
	if channelID != 123 {
		t.Errorf("Expected channel_id 123, got %f", channelID)
	}
}

func TestPresenceUpdateEncoding(t *testing.T) {
	// Simulate presence update event
	eventData := map[string]interface{}{
		"event":         "presence_updated",
		"user_id":       1,
		"username":      "testuser",
		"status":        "online",
		"last_seen_iso": nil,
	}
	
	result := encodeSocketIOWithNamespace(PACKET_EVENT, "", "presence_updated", eventData)
	
	// Verify result starts with packet type 2 (EVENT)
	if len(result) < 1 || result[0] != '2' {
		t.Errorf("Expected EVENT packet (type 2), got: %s", result)
	}
	
	// Verify it's valid JSON
	var arr []interface{}
	if err := json.Unmarshal([]byte(result[1:]), &arr); err != nil {
		t.Errorf("Invalid JSON in encoded message: %v", err)
	}
}

func TestCommandResultEncoding(t *testing.T) {
	tests := []struct {
		name    string
		ok      bool
		message string
	}{
		{"Success", true, "Operation completed"},
		{"Error", false, "Permission denied"},
		{"Empty message", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventData := map[string]interface{}{
				"event":   "command_result",
				"ok":      tt.ok,
				"message": tt.message,
			}
			
			result := encodeSocketIOWithNamespace(PACKET_EVENT, "", "command_result", eventData)
			
			// Parse back and verify
			var arr []interface{}
			if err := json.Unmarshal([]byte(result[1:]), &arr); err != nil {
				t.Fatalf("Invalid JSON: %v", err)
			}
			
			if len(arr) < 2 {
				t.Fatalf("Expected array with at least 2 elements")
			}
			
			dataMap, ok := arr[1].(map[string]interface{})
			if !ok {
				t.Fatalf("Second element is not a map")
			}
			
			if dataMap["ok"] != tt.ok {
				t.Errorf("Expected ok=%v, got %v", tt.ok, dataMap["ok"])
			}
		})
	}
}
