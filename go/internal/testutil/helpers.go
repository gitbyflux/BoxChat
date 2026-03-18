// Package testutil предоставляет общие вспомогательные функции для тестирования.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Константы для тестов
// ============================================================================

const (
	// Cookie names
	CookieUID        = "boxchat_uid"
	CookieUname      = "boxchat_uname"
	CookieAuthMode   = "boxchat_auth_mode"
	CookieRemember   = "boxchat_remember"
	CookieSession    = "boxchat_session"

	// Test credentials
	TestPassword        = "password123"
	TestAdminPassword   = "AdminPass123!"
	TestSuperPassword   = "SuperPass123!"

	// HTTP status codes
	StatusOK                  = http.StatusOK
	StatusCreated             = http.StatusCreated
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusForbidden           = http.StatusForbidden
	StatusNotFound            = http.StatusNotFound
	StatusConflict            = http.StatusConflict
	StatusFound               = http.StatusFound
	StatusNoContent           = http.StatusNoContent

	// Минимальная/максимальная длина username
	MinUsernameLength = 3
	MaxUsernameLength = 30

	// Минимальная длина пароля
	MinPasswordLength = 8

	// Количество cookie для сессии
	MinSessionCookies = 3
)

// ============================================================================
// Setup/Teardown базы данных
// ============================================================================

// SetupTestDB инициализирует тестовую базу данных в временной директории.
// Возвращает функцию cleanup для очистки после теста.
func SetupTestDB(t *testing.T) (*config.Config, func()) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	gin.SetMode(gin.TestMode)

	cleanup := func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}

	return cfg, cleanup
}

// SetupTestDBWithAdmin инициализирует БД с заданным паролем администратора.
func SetupTestDBWithAdmin(t *testing.T, adminPassword string) (*config.Config, func()) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")
	if adminPassword != "" {
		os.Setenv("ADMIN_PASSWORD", adminPassword)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	gin.SetMode(gin.TestMode)

	cleanup := func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
		os.Unsetenv("ADMIN_PASSWORD")
	}

	return cfg, cleanup
}

// InitInMemoryDB инициализирует базу данных в памяти для быстрых тестов.
// Возвращает функцию cleanup для закрытия соединения.
func InitInMemoryDB(t *testing.T) func() {
	t.Helper()

	os.Setenv("SQLALCHEMY_DATABASE_URI", "file::memory:?cache=shared")
	os.Setenv("SECRET_KEY", "test_secret_key_12345678901234567890123456789012")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize in-memory database: %v", err)
	}

	gin.SetMode(gin.TestMode)

	cleanup := func() {
		if database.DB != nil {
			sqlDB, _ := database.DB.DB()
			sqlDB.Close()
		}
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}

	return cleanup
}

// ============================================================================
// Хелперы для работы с паролями
// ============================================================================

// HashPassword создаёт bcrypt хеш пароля для тестов.
func HashPassword(t *testing.T, password string) string {
	t.Helper()

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

// ComparePassword проверяет соответствие пароля хешу.
func ComparePassword(t *testing.T, password, hash string) bool {
	t.Helper()

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ============================================================================
// Хелперы для создания тестовых данных
// ============================================================================

// CreateUser создаёт тестового пользователя.
func CreateUser(t *testing.T, username string, isSuperuser, isBanned bool) *models.User {
	t.Helper()

	hashedPassword := HashPassword(t, TestPassword)
	user := models.User{
		Username:       username,
		Password:       hashedPassword,
		PresenceStatus: "offline",
		IsSuperuser:    isSuperuser,
		IsBanned:       isBanned,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	return &user
}

// CreateUserWithPassword создаёт пользователя с заданным паролем.
func CreateUserWithPassword(t *testing.T, username, password string) *models.User {
	t.Helper()

	hashedPassword := HashPassword(t, password)
	user := models.User{
		Username:       username,
		Password:       hashedPassword,
		PresenceStatus: "offline",
	}

	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	return &user
}

// CreateRoom создаёт тестовую комнату.
func CreateRoom(t *testing.T, name, roomType string, ownerID *uint) *models.Room {
	t.Helper()

	room := models.Room{
		Name:    name,
		Type:    roomType,
		OwnerID: ownerID,
	}

	if err := database.DB.Create(&room).Error; err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	return &room
}

// CreateChannel создаёт канал в комнате.
func CreateChannel(t *testing.T, name string, roomID uint) *models.Channel {
	t.Helper()

	channel := models.Channel{
		Name:   name,
		RoomID: roomID,
	}

	if err := database.DB.Create(&channel).Error; err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	return &channel
}

// CreateMember создаёт участника в комнате.
func CreateMember(t *testing.T, userID, roomID uint, role string) *models.Member {
	t.Helper()

	member := models.Member{
		UserID: userID,
		RoomID: roomID,
		Role:   role,
	}

	if err := database.DB.Create(&member).Error; err != nil {
		t.Fatalf("Failed to create member: %v", err)
	}

	return &member
}

// CreateMemberWithMute создаёт участника с mute.
func CreateMemberWithMute(t *testing.T, userID, roomID uint, role string, mutedUntil *time.Time) *models.Member {
	t.Helper()

	member := models.Member{
		UserID:     userID,
		RoomID:     roomID,
		Role:       role,
		MutedUntil: mutedUntil,
	}

	if err := database.DB.Create(&member).Error; err != nil {
		t.Fatalf("Failed to create member: %v", err)
	}

	return &member
}

// CreateRole создаёт роль в комнате.
func CreateRole(t *testing.T, roomID uint, name, tag string, permissions []string, isSystem bool) *models.Role {
	t.Helper()

	role := models.Role{
		RoomID:                   roomID,
		Name:                     name,
		MentionTag:               tag,
		IsSystem:                 isSystem,
		CanBeMentionedByEveryone: false,
	}

	if len(permissions) > 0 {
		permsJSON, err := json.Marshal(permissions)
		if err != nil {
			t.Fatalf("Failed to marshal permissions: %v", err)
		}
		role.PermissionsJSON = string(permsJSON)
	} else {
		role.PermissionsJSON = "[]"
	}

	if err := database.DB.Create(&role).Error; err != nil {
		t.Fatalf("Failed to create role: %v", err)
	}

	return &role
}

// CreateMessage создаёт сообщение в канале.
func CreateMessage(t *testing.T, content string, userID, channelID uint, messageType string) *models.Message {
	t.Helper()

	if messageType == "" {
		messageType = "text"
	}

	message := models.Message{
		Content:     content,
		UserID:      userID,
		ChannelID:   channelID,
		MessageType: messageType,
	}

	if err := database.DB.Create(&message).Error; err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	return &message
}

// CreateRoomBan создаёт бан в комнате.
func CreateRoomBan(t *testing.T, roomID, userID uint, reason string) *models.RoomBan {
	t.Helper()

	ban := models.RoomBan{
		RoomID: roomID,
		UserID: userID,
		Reason: reason,
	}

	if err := database.DB.Create(&ban).Error; err != nil {
		t.Fatalf("Failed to create room ban: %v", err)
	}

	return &ban
}

// CreateFriendRequest создаёт запрос в друзья.
func CreateFriendRequest(t *testing.T, fromUserID, toUserID uint, status string) *models.FriendRequest {
	t.Helper()

	request := models.FriendRequest{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Status:     status,
	}

	if err := database.DB.Create(&request).Error; err != nil {
		t.Fatalf("Failed to create friend request: %v", err)
	}

	return &request
}

// CreateFriendship создаёт дружбу между пользователями.
func CreateFriendship(t *testing.T, userLowID, userHighID uint) *models.Friendship {
	t.Helper()

	friendship := models.Friendship{
		UserLowID:  userLowID,
		UserHighID: userHighID,
	}

	if err := database.DB.Create(&friendship).Error; err != nil {
		t.Fatalf("Failed to create friendship: %v", err)
	}

	return &friendship
}

// ============================================================================
// Хелперы для HTTP запросов
// ============================================================================

// AddAuthCookie добавляет cookie аутентификации к запросу.
func AddAuthCookie(req *http.Request, userID uint) {
	req.AddCookie(&http.Cookie{
		Name:  CookieUID,
		Value: fmt.Sprintf("%d", userID),
	})
}

// AddAuthCookies добавляет все необходимые cookie для сессии.
func AddAuthCookies(req *http.Request, userID uint, username, authMode string) {
	req.AddCookie(&http.Cookie{
		Name:  CookieUID,
		Value: fmt.Sprintf("%d", userID),
	})
	req.AddCookie(&http.Cookie{
		Name:  CookieUname,
		Value: username,
	})
	if authMode != "" {
		req.AddCookie(&http.Cookie{
			Name:  CookieAuthMode,
			Value: authMode,
		})
	}
}

// CreateJSONRequest создаёт HTTP запрос с JSON телом.
func CreateJSONRequest(method, url string, body interface{}) (*http.Request, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// CreateFormRequest создаёт HTTP запрос с form-urlencoded телом.
func CreateFormRequest(method, url string, formData string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBufferString(formData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

// CreateMultipartRequest создаёт multipart/form-data запрос.
func CreateMultipartRequest(t *testing.T, method, url string, fields map[string]string, files map[string][]byte) (*http.Request, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add fields
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("Failed to write field %s: %v", key, err)
		}
	}

	// Add files
	for key, data := range files {
		part, err := writer.CreateFormFile(key, "testfile")
		if err != nil {
			t.Fatalf("Failed to create form file %s: %v", key, err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("Failed to write file %s: %v", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}

	req, err := http.NewRequest(method, "/test", body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

// ============================================================================
// Хелперы для проверки ответов
// ============================================================================

// AssertStatus проверяет HTTP статус код.
func AssertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("Expected status %d, got %d", want, got)
	}
}

// AssertJSONResponse проверяет, что ответ является валидным JSON.
func AssertJSONResponse(t *testing.T, body *bytes.Buffer, target interface{}) {
	t.Helper()

	if err := json.Unmarshal(body.Bytes(), target); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
}

// AssertCookie проверяет наличие и значение cookie.
func AssertCookie(t *testing.T, cookies []*http.Cookie, name, expectedValue string) {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			if expectedValue != "" && cookie.Value != expectedValue {
				t.Errorf("Cookie %s: expected value %q, got %q", name, expectedValue, cookie.Value)
			}
			return
		}
	}

	t.Errorf("Cookie %s not found", name)
}

// AssertCookieExists проверяет наличие cookie.
func AssertCookieExists(t *testing.T, cookies []*http.Cookie, name string) {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return
		}
	}

	t.Errorf("Cookie %s not found", name)
}

// AssertCookieCleared проверяет, что cookie очищена (пустое значение).
func AssertCookieCleared(t *testing.T, cookies []*http.Cookie, name string) {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			if cookie.Value != "" {
				t.Errorf("Cookie %s should be cleared, got value %q", name, cookie.Value)
			}
			return
		}
	}

	t.Errorf("Cookie %s not found", name)
}

// AssertMapValue проверяет значение в map[string]interface{}.
func AssertMapValue(t *testing.T, m map[string]interface{}, key string, expectedValue interface{}) {
	t.Helper()

	value, ok := m[key]
	if !ok {
		t.Errorf("Key %q not found in response", key)
		return
	}

	if value != expectedValue {
		t.Errorf("Key %q: expected %v, got %v", key, expectedValue, value)
	}
}

// AssertMapBool проверяет, что значение в map является bool.
func AssertMapBool(t *testing.T, m map[string]interface{}, key string, expected bool) {
	t.Helper()

	value, ok := m[key]
	if !ok {
		t.Errorf("Key %q not found in response", key)
		return
	}

	boolValue, ok := value.(bool)
	if !ok {
		t.Errorf("Key %q is not a bool: %T", key, value)
		return
	}

	if boolValue != expected {
		t.Errorf("Key %q: expected %v, got %v", key, expected, boolValue)
	}
}

// AssertMapString проверяет, что значение в map является string.
func AssertMapString(t *testing.T, m map[string]interface{}, key, expected string) {
	t.Helper()

	value, ok := m[key]
	if !ok {
		t.Errorf("Key %q not found in response", key)
		return
	}

	strValue, ok := value.(string)
	if !ok {
		t.Errorf("Key %q is not a string: %T", key, value)
		return
	}

	if strValue != expected {
		t.Errorf("Key %q: expected %q, got %q", key, expected, strValue)
	}
}

// AssertMapObject проверяет, что значение в map является объектом.
func AssertMapObject(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
	t.Helper()

	value, ok := m[key]
	if !ok {
		t.Errorf("Key %q not found in response", key)
		return nil
	}

	objValue, ok := value.(map[string]interface{})
	if !ok {
		t.Errorf("Key %q is not an object: %T", key, value)
		return nil
	}

	return objValue
}

// AssertMapArray проверяет, что значение в map является массивом.
func AssertMapArray(t *testing.T, m map[string]interface{}, key string) []interface{} {
	t.Helper()

	value, ok := m[key]
	if !ok {
		t.Errorf("Key %q not found in response", key)
		return nil
	}

	arrValue, ok := value.([]interface{})
	if !ok {
		t.Errorf("Key %q is not an array: %T", key, value)
		return nil
	}

	return arrValue
}

// AssertArrayLength проверяет длину массива.
func AssertArrayLength(t *testing.T, arr []interface{}, expected int) {
	t.Helper()

	if len(arr) != expected {
		t.Errorf("Expected array length %d, got %d", expected, len(arr))
	}
}

// AssertNotEmpty проверяет, что значение не пустое.
func AssertNotEmpty(t *testing.T, value interface{}, name string) {
	t.Helper()

	switch v := value.(type) {
	case string:
		if v == "" {
			t.Errorf("%s should not be empty", name)
		}
	case int:
		if v == 0 {
			t.Errorf("%s should not be 0", name)
		}
	case uint:
		if v == 0 {
			t.Errorf("%s should not be 0", name)
		}
	case nil:
		t.Errorf("%s should not be nil", name)
	}
}

// ============================================================================
// Хелперы для WebSocket тестов
// ============================================================================

// CreateTestRecorder создаёт httptest.ResponseRecorder для тестов.
func CreateTestRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// CreateTestContext создаёт Gin контекст для тестов.
func CreateTestContext(w *httptest.ResponseRecorder, method, url string, body []byte) (*gin.Context, *http.Request) {
	req, _ := http.NewRequest(method, url, bytes.NewBuffer(body))
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, req
}

// ============================================================================
// Утилиты
// ============================================================================

// SkipIfShort пропускает тест в режиме short.
func SkipIfShort(t *testing.T, reason string) {
	t.Helper()
	if testing.Short() {
		t.Skipf("Skipping test in short mode: %s", reason)
	}
}

// RunWithTimeout запускает тест с таймаутом.
func RunWithTimeout(t *testing.T, timeoutMs int, fn func()) {
	t.Helper()

	done := make(chan bool)
	go func() {
		fn()
		done <- true
	}()

	select {
	case <-done:
		// Успех
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		t.Fatalf("Test timed out after %d ms", timeoutMs)
	}
}
