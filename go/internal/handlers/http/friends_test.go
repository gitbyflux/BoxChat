package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/testutil"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Helper Functions for Friends Tests
// ============================================================================

func setupFriendsTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	cfg, dbCleanup := testutil.SetupTestDB(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	authHandler := NewAuthHandler(cfg)
	apiHandler := NewAPIHandler(cfg)

	authHandler.RegisterRoutes(router)
	apiHandler.RegisterRoutes(router)
	apiHandler.RegisterFriendsRoutes(router)

	return cfg, router, dbCleanup
}

func hashPasswordFriends(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

func createFriendsUser(t *testing.T, username string) *models.User {
	t.Helper()
	hashedPassword := hashPasswordFriends(t, "password123")
	user := models.User{
		Username: username,
		Password: hashedPassword,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	return &user
}

func createFriendship(t *testing.T, userID1, userID2 uint) {
	t.Helper()
	low, high := minMax(userID1, userID2)
	friendship := models.Friendship{
		UserLowID:  low,
		UserHighID: high,
	}
	if err := database.DB.Create(&friendship).Error; err != nil {
		t.Fatalf("Failed to create friendship: %v", err)
	}
}

func createFriendRequest(t *testing.T, fromUserID, toUserID uint, status string) *models.FriendRequest {
	t.Helper()
	request := models.FriendRequest{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Status:     status,
		CreatedAt:  time.Now(),
	}
	if err := database.DB.Create(&request).Error; err != nil {
		t.Fatalf("Failed to create friend request: %v", err)
	}
	return &request
}

func addAuthCookieFriends(req *http.Request, userID uint) {
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", userID),
	})
}

// ============================================================================
// FriendStatus Tests
// ============================================================================

func TestFriendStatus_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "friendstatus1")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/friends/status/%d", user.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestFriendStatus_Self(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "friendstatus2")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/friends/status/%d", user.ID), nil)
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "self" {
		t.Errorf("Expected status 'self', got %v", response["status"])
	}
}

func TestFriendStatus_Friends(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "friendstatus3a")
	user2 := createFriendsUser(t, "friendstatus3b")
	createFriendship(t, user1.ID, user2.ID)

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/friends/status/%d", user2.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "friends" {
		t.Errorf("Expected status 'friends', got %v", response["status"])
	}
}

func TestFriendStatus_PendingIncoming(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "friendstatus4a")
	user2 := createFriendsUser(t, "friendstatus4b")
	createFriendRequest(t, user1.ID, user2.ID, "pending")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/friends/status/%d", user1.ID), nil)
	addAuthCookieFriends(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "pending" {
		t.Errorf("Expected status 'pending', got %v", response["status"])
	}
	if response["direction"] != "incoming" {
		t.Errorf("Expected direction 'incoming', got %v", response["direction"])
	}
}

func TestFriendStatus_PendingOutgoing(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "friendstatus5a")
	user2 := createFriendsUser(t, "friendstatus5b")
	createFriendRequest(t, user1.ID, user2.ID, "pending")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/friends/status/%d", user2.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "pending" {
		t.Errorf("Expected status 'pending', got %v", response["status"])
	}
	if response["direction"] != "outgoing" {
		t.Errorf("Expected direction 'outgoing', got %v", response["direction"])
	}
}

func TestFriendStatus_None(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "friendstatus6a")
	user2 := createFriendsUser(t, "friendstatus6b")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/friends/status/%d", user2.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "none" {
		t.Errorf("Expected status 'none', got %v", response["status"])
	}
}

// ============================================================================
// SendFriendRequest Tests
// ============================================================================

func TestSendFriendRequest_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "sendrequest1")

	requestData := map[string]string{
		"username": user.Username,
	}
	jsonData, _ := json.Marshal(requestData)

	req, _ := http.NewRequest("POST", "/api/v1/friends/request", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestSendFriendRequest_UserNotFound(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "sendrequest2")

	requestData := map[string]string{
		"username": "nonexistent",
	}
	jsonData, _ := json.Marshal(requestData)

	req, _ := http.NewRequest("POST", "/api/v1/friends/request", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestSendFriendRequest_ToSelf(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "sendrequest3")

	requestData := map[string]string{
		"username": user.Username,
	}
	jsonData, _ := json.Marshal(requestData)

	req, _ := http.NewRequest("POST", "/api/v1/friends/request", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSendFriendRequest_AlreadyFriends(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "sendrequest4a")
	user2 := createFriendsUser(t, "sendrequest4b")
	createFriendship(t, user1.ID, user2.ID)

	requestData := map[string]string{
		"username": user2.Username,
	}
	jsonData, _ := json.Marshal(requestData)

	req, _ := http.NewRequest("POST", "/api/v1/friends/request", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestSendFriendRequest_RequestExists(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "sendrequest5a")
	user2 := createFriendsUser(t, "sendrequest5b")
	createFriendRequest(t, user1.ID, user2.ID, "pending")

	requestData := map[string]string{
		"username": user2.Username,
	}
	jsonData, _ := json.Marshal(requestData)

	req, _ := http.NewRequest("POST", "/api/v1/friends/request", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestSendFriendRequest_Success(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "sendrequest6a")
	user2 := createFriendsUser(t, "sendrequest6b")

	requestData := map[string]string{
		"username": user2.Username,
	}
	jsonData, _ := json.Marshal(requestData)

	req, _ := http.NewRequest("POST", "/api/v1/friends/request", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}
}

// ============================================================================
// GetFriendRequests Tests
// ============================================================================

func TestGetFriendRequests_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/friends/requests", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetFriendRequests_Empty(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "getrequests1")

	req, _ := http.NewRequest("GET", "/api/v1/friends/requests", nil)
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	incoming, ok := response["incoming"].([]interface{})
	if !ok {
		t.Fatal("Expected incoming array")
	}
	if len(incoming) != 0 {
		t.Errorf("Expected 0 incoming requests, got %d", len(incoming))
	}

	outgoing, ok := response["outgoing"].([]interface{})
	if !ok {
		t.Fatal("Expected outgoing array")
	}
	if len(outgoing) != 0 {
		t.Errorf("Expected 0 outgoing requests, got %d", len(outgoing))
	}
}

func TestGetFriendRequests_WithRequests(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "getrequests2a")
	user2 := createFriendsUser(t, "getrequests2b")
	user3 := createFriendsUser(t, "getrequests2c")

	// Incoming request
	createFriendRequest(t, user2.ID, user1.ID, "pending")
	// Outgoing request
	createFriendRequest(t, user1.ID, user3.ID, "pending")

	req, _ := http.NewRequest("GET", "/api/v1/friends/requests", nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	incoming, ok := response["incoming"].([]interface{})
	if !ok {
		t.Fatal("Expected incoming array")
	}
	if len(incoming) != 1 {
		t.Errorf("Expected 1 incoming request, got %d", len(incoming))
	}

	outgoing, ok := response["outgoing"].([]interface{})
	if !ok {
		t.Fatal("Expected outgoing array")
	}
	if len(outgoing) != 1 {
		t.Errorf("Expected 1 outgoing request, got %d", len(outgoing))
	}
}

// ============================================================================
// RespondToFriendRequest Tests
// ============================================================================

func TestRespondToFriendRequest_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "respondrequest1")
	request := createFriendRequest(t, user.ID, user.ID, "pending")

	responseData := map[string]string{
		"status": "accept",
	}
	jsonData, _ := json.Marshal(responseData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/friends/requests/%d/respond", request.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRespondToFriendRequest_NotFound(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "respondrequest2")

	responseData := map[string]string{
		"status": "accept",
	}
	jsonData, _ := json.Marshal(responseData)

	req, _ := http.NewRequest("POST", "/api/v1/friends/requests/99999/respond", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestRespondToFriendRequest_NotAuthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "respondrequest3a")
	user2 := createFriendsUser(t, "respondrequest3b")
	request := createFriendRequest(t, user1.ID, user2.ID, "pending")

	responseData := map[string]string{
		"status": "accept",
	}
	jsonData, _ := json.Marshal(responseData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/friends/requests/%d/respond", request.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user1.ID) // Sender trying to respond
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestRespondToFriendRequest_AlreadyResponded(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "respondrequest4a")
	user2 := createFriendsUser(t, "respondrequest4b")
	request := createFriendRequest(t, user1.ID, user2.ID, "accepted")

	responseData := map[string]string{
		"status": "accept",
	}
	jsonData, _ := json.Marshal(responseData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/friends/requests/%d/respond", request.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestRespondToFriendRequest_AcceptSuccess(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "respondrequest5a")
	user2 := createFriendsUser(t, "respondrequest5b")
	request := createFriendRequest(t, user1.ID, user2.ID, "pending")

	responseData := map[string]string{
		"status": "accept",
	}
	jsonData, _ := json.Marshal(responseData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/friends/requests/%d/respond", request.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify friendship created
	var friendship models.Friendship
	low, high := minMax(user1.ID, user2.ID)
	result := database.DB.Where("user_low_id = ? AND user_high_id = ?", low, high).First(&friendship)
	if result.Error != nil {
		t.Error("Friendship should be created")
	}
}

func TestRespondToFriendRequest_DeclineSuccess(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "respondrequest6a")
	user2 := createFriendsUser(t, "respondrequest6b")
	request := createFriendRequest(t, user1.ID, user2.ID, "pending")

	responseData := map[string]string{
		"status": "decline",
	}
	jsonData, _ := json.Marshal(responseData)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/friends/requests/%d/respond", request.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieFriends(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify request declined
	var updatedRequest models.FriendRequest
	database.DB.First(&updatedRequest, request.ID)
	if updatedRequest.Status != "declined" {
		t.Errorf("Expected status 'declined', got %s", updatedRequest.Status)
	}
}

// ============================================================================
// CancelFriendRequest Tests
// ============================================================================

func TestCancelFriendRequest_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "cancelrequest1")
	request := createFriendRequest(t, user.ID, user.ID, "pending")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/friends/requests/%d", request.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestCancelFriendRequest_NotFound(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "cancelrequest2")

	req, _ := http.NewRequest("DELETE", "/api/v1/friends/requests/99999", nil)
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestCancelFriendRequest_NotSender(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "cancelrequest3a")
	user2 := createFriendsUser(t, "cancelrequest3b")
	request := createFriendRequest(t, user1.ID, user2.ID, "pending")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/friends/requests/%d", request.ID), nil)
	addAuthCookieFriends(req, user2.ID) // Recipient trying to cancel
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestCancelFriendRequest_AlreadyResponded(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "cancelrequest4a")
	user2 := createFriendsUser(t, "cancelrequest4b")
	request := createFriendRequest(t, user1.ID, user2.ID, "accepted")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/friends/requests/%d", request.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestCancelFriendRequest_Success(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "cancelrequest5a")
	user2 := createFriendsUser(t, "cancelrequest5b")
	request := createFriendRequest(t, user1.ID, user2.ID, "pending")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/friends/requests/%d", request.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify request deleted
	var deletedRequest models.FriendRequest
	result := database.DB.First(&deletedRequest, request.ID)
	if result.Error == nil {
		t.Error("CancelFriendRequest() should delete the request")
	}
}

// ============================================================================
// GetFriends Tests
// ============================================================================

func TestGetFriends_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/friends", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetFriends_Empty(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "getfriends1")

	req, _ := http.NewRequest("GET", "/api/v1/friends", nil)
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	friends, ok := response["friends"].([]interface{})
	if !ok {
		t.Fatal("Expected friends array")
	}
	if len(friends) != 0 {
		t.Errorf("Expected 0 friends, got %d", len(friends))
	}
}

func TestGetFriends_WithFriends(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "getfriends2a")
	user2 := createFriendsUser(t, "getfriends2b")
	createFriendship(t, user1.ID, user2.ID)

	req, _ := http.NewRequest("GET", "/api/v1/friends", nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	friends, ok := response["friends"].([]interface{})
	if !ok {
		t.Fatal("Expected friends array")
	}
	if len(friends) < 1 {
		t.Errorf("Expected at least 1 friend, got %d", len(friends))
	}
}

// ============================================================================
// RemoveFriend Tests
// ============================================================================

func TestRemoveFriend_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "removefriend1")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/friends/%d", user.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRemoveFriend_NotFound(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "removefriend2")

	req, _ := http.NewRequest("DELETE", "/api/v1/friends/99999", nil)
	addAuthCookieFriends(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestRemoveFriend_Success(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "removefriend3a")
	user2 := createFriendsUser(t, "removefriend3b")
	createFriendship(t, user1.ID, user2.ID)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/friends/%d", user2.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify friendship deleted
	var friendship models.Friendship
	low, high := minMax(user1.ID, user2.ID)
	result := database.DB.Where("user_low_id = ? AND user_high_id = ?", low, high).First(&friendship)
	if result.Error == nil {
		t.Error("RemoveFriend() should delete the friendship")
	}
}

// ============================================================================
// CreateDMRoom Tests
// ============================================================================

func TestCreateDMRoom_Unauthorized(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user := createFriendsUser(t, "createdm1")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dm/%d/create", user.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestCreateDMRoom_NotFriends(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "createdm2a")
	user2 := createFriendsUser(t, "createdm2b")

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dm/%d/create", user2.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestCreateDMRoom_AlreadyExists(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "createdm3a")
	user2 := createFriendsUser(t, "createdm3b")
	createFriendship(t, user1.ID, user2.ID)

	// Create existing DM
	dm := models.Room{
		Name:     "DM",
		Type:     "dm",
		IsPublic: false,
		OwnerID:  &user1.ID,
	}
	database.DB.Create(&dm)
	member1 := models.Member{UserID: user1.ID, RoomID: dm.ID, Role: "owner"}
	member2 := models.Member{UserID: user2.ID, RoomID: dm.ID, Role: "member"}
	database.DB.Create(&member1)
	database.DB.Create(&member2)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dm/%d/create", user2.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["existing"] != true {
		t.Error("Expected existing=true")
	}
}

func TestCreateDMRoom_Success(t *testing.T) {
	_, router, cleanup := setupFriendsTestDB(t)
	defer cleanup()

	user1 := createFriendsUser(t, "createdm4a")
	user2 := createFriendsUser(t, "createdm4b")
	createFriendship(t, user1.ID, user2.ID)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dm/%d/create", user2.ID), nil)
	addAuthCookieFriends(req, user1.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}
	if response["existing"] != false {
		t.Error("Expected existing=false")
	}
}
