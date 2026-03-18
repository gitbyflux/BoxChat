package http

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

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// Helper Functions for Stickers Tests
// ============================================================================

func setupStickersTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	database.ResetForTesting()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	os.Setenv("SQLALCHEMY_DATABASE_URI", "sqlite:///"+dbPath)
	os.Setenv("SECRET_KEY", "test_secret_key_stickers")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := database.Init(cfg); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	utils.InitExtensions(cfg)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	authHandler := NewAuthHandler(cfg)
	apiHandler := NewAPIHandler(cfg)

	authHandler.RegisterRoutes(router)
	apiHandler.RegisterRoutes(router)
	apiHandler.RegisterStickersRoutes(router)

	cleanup := func() {
		os.Unsetenv("SQLALCHEMY_DATABASE_URI")
		os.Unsetenv("SECRET_KEY")
	}

	return cfg, router, cleanup
}

func hashPasswordStickers(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

func createStickersUser(t *testing.T, username string, isSuperuser bool) *models.User {
	t.Helper()
	hashedPassword := hashPasswordStickers(t, "password123")
	user := models.User{
		Username:    username,
		Password:    hashedPassword,
		IsSuperuser: isSuperuser,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	return &user
}

func createStickerPack(t *testing.T, ownerID uint, name string) *models.StickerPack {
	t.Helper()
	pack := models.StickerPack{
		Name:    name,
		OwnerID: ownerID,
	}
	if err := database.DB.Create(&pack).Error; err != nil {
		t.Fatalf("Failed to create sticker pack: %v", err)
	}
	return &pack
}

func createSticker(t *testing.T, packID, ownerID uint, name, fileURL string) *models.Sticker {
	t.Helper()
	sticker := models.Sticker{
		Name:    name,
		FileURL: fileURL,
		PackID:  packID,
		OwnerID: ownerID,
	}
	if err := database.DB.Create(&sticker).Error; err != nil {
		t.Fatalf("Failed to create sticker: %v", err)
	}
	return &sticker
}

func addAuthCookieStickers(req *http.Request, userID uint) {
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", userID),
	})
}

func createMultipartStickerRequest(t *testing.T, method, url, fieldName, filename string, fileContent []byte, stickerName string, userID *uint) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}

	if stickerName != "" {
		writer.WriteField("name", stickerName)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}

	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if userID != nil {
		addAuthCookieStickers(req, *userID)
	}

	w := httptest.NewRecorder()
	return req, w
}

// ============================================================================
// CreateStickerPack Tests
// ============================================================================

func TestCreateStickerPack_Unauthorized(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()

	packData := map[string]string{
		"name": "Test Pack",
	}
	jsonData, _ := json.Marshal(packData)

	req, _ := http.NewRequest("POST", "/api/v1/sticker_packs", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestCreateStickerPack_NoName(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickeruser1", false)

	packData := map[string]string{}
	jsonData, _ := json.Marshal(packData)

	req, _ := http.NewRequest("POST", "/api/v1/sticker_packs", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCreateStickerPack_Success(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickeruser2", false)

	packData := map[string]string{
		"name":       "My Stickers",
		"icon_emoji": "😀",
	}
	jsonData, _ := json.Marshal(packData)

	req, _ := http.NewRequest("POST", "/api/v1/sticker_packs", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieStickers(req, user.ID)
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

	pack, ok := response["pack"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected pack in response")
	}

	if pack["name"] != "My Stickers" {
		t.Errorf("Expected name 'My Stickers', got %v", pack["name"])
	}
}

// ============================================================================
// GetUserStickerPacks Tests
// ============================================================================

func TestGetUserStickerPacks_Unauthorized(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/sticker_packs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetUserStickerPacks_Empty(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickeruser3", false)

	req, _ := http.NewRequest("GET", "/api/v1/sticker_packs", nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	packs, ok := response["packs"].([]interface{})
	if !ok {
		t.Fatal("Expected packs array")
	}
	if len(packs) != 0 {
		t.Errorf("Expected 0 packs, got %d", len(packs))
	}
}

func TestGetUserStickerPacks_WithPacks(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickeruser4", false)
	createStickerPack(t, user.ID, "Pack 1")
	createStickerPack(t, user.ID, "Pack 2")

	req, _ := http.NewRequest("GET", "/api/v1/sticker_packs", nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	packs, ok := response["packs"].([]interface{})
	if !ok {
		t.Fatal("Expected packs array")
	}
	if len(packs) < 2 {
		t.Errorf("Expected at least 2 packs, got %d", len(packs))
	}
}

// ============================================================================
// GetStickerPack Tests
// ============================================================================

func TestGetStickerPack_Unauthorized(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickeruser5", false)
	pack := createStickerPack(t, user.ID, "Test Pack")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetStickerPack_NotFound(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickeruser6", false)

	req, _ := http.NewRequest("GET", "/api/v1/sticker_packs/99999", nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetStickerPack_AccessDenied(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user1 := createStickersUser(t, "stickeruser7a", false)
	user2 := createStickersUser(t, "stickeruser7b", false)
	pack := createStickerPack(t, user1.ID, "Private Pack")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), nil)
	addAuthCookieStickers(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestGetStickerPack_SuperuserAccess(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user1 := createStickersUser(t, "stickeruser8a", false)
	user2 := createStickersUser(t, "stickeruser8b", true) // Superuser
	pack := createStickerPack(t, user1.ID, "Any Pack")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), nil)
	addAuthCookieStickers(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetStickerPack_OwnerSuccess(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickeruser9", false)
	pack := createStickerPack(t, user.ID, "My Pack")
	createSticker(t, pack.ID, user.ID, "smile", "/stickers/smile.png")

	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	packData, ok := response["pack"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected pack in response")
	}

	if packData["name"] != "My Pack" {
		t.Errorf("Expected name 'My Pack', got %v", packData["name"])
	}
}

// ============================================================================
// UpdateStickerPack Tests
// ============================================================================

func TestUpdateStickerPack_Unauthorized(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickerupdate1", false)
	pack := createStickerPack(t, user.ID, "Old Pack")

	packData := map[string]string{
		"name": "New Pack",
	}
	jsonData, _ := json.Marshal(packData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestUpdateStickerPack_NotFound(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickerupdate2", false)

	packData := map[string]string{
		"name": "New Pack",
	}
	jsonData, _ := json.Marshal(packData)

	req, _ := http.NewRequest("PATCH", "/api/v1/sticker_packs/99999", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestUpdateStickerPack_AccessDenied(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user1 := createStickersUser(t, "stickerupdate3a", false)
	user2 := createStickersUser(t, "stickerupdate3b", false)
	pack := createStickerPack(t, user1.ID, "Pack")

	packData := map[string]string{
		"name": "New Pack",
	}
	jsonData, _ := json.Marshal(packData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieStickers(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestUpdateStickerPack_Success(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickerupdate4", false)
	pack := createStickerPack(t, user.ID, "Old Pack")

	packData := map[string]string{
		"name":       "Updated Pack",
		"icon_emoji": "🎉",
	}
	jsonData, _ := json.Marshal(packData)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
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
// DeleteStickerPack Tests
// ============================================================================

func TestDeleteStickerPack_Unauthorized(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickerdelete1", false)
	pack := createStickerPack(t, user.ID, "Pack")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestDeleteStickerPack_NotFound(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickerdelete2", false)

	req, _ := http.NewRequest("DELETE", "/api/v1/sticker_packs/99999", nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDeleteStickerPack_AccessDenied(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user1 := createStickersUser(t, "stickerdelete3a", false)
	user2 := createStickersUser(t, "stickerdelete3b", false)
	pack := createStickerPack(t, user1.ID, "Pack")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), nil)
	addAuthCookieStickers(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestDeleteStickerPack_Success(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "stickerdelete4", false)
	pack := createStickerPack(t, user.ID, "Pack")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/sticker_packs/%d", pack.ID), nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify pack deleted
	var deletedPack models.StickerPack
	result := database.DB.First(&deletedPack, pack.ID)
	if result.Error == nil {
		t.Error("DeleteStickerPack() should delete the pack")
	}
}

// ============================================================================
// AddSticker Tests
// ============================================================================

func TestAddSticker_Unauthorized(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "addsticker1", false)
	pack := createStickerPack(t, user.ID, "Pack")

	fileContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	req, w := createMultipartStickerRequest(t, "POST", fmt.Sprintf("/api/v1/sticker_packs/%d/stickers", pack.ID), "sticker_file", "sticker.png", fileContent, "smile", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAddSticker_PackNotFound(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "addsticker2", false)

	fileContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	req, w := createMultipartStickerRequest(t, "POST", "/api/v1/sticker_packs/99999/stickers", "sticker_file", "sticker.png", fileContent, "smile", &user.ID)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAddSticker_AccessDenied(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user1 := createStickersUser(t, "addsticker3a", false)
	user2 := createStickersUser(t, "addsticker3b", false)
	pack := createStickerPack(t, user1.ID, "Pack")

	fileContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	req, w := createMultipartStickerRequest(t, "POST", fmt.Sprintf("/api/v1/sticker_packs/%d/stickers", pack.ID), "sticker_file", "sticker.png", fileContent, "smile", &user2.ID)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestAddSticker_NoFile(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "addsticker4", false)
	pack := createStickerPack(t, user.ID, "Pack")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/sticker_packs/%d/stickers", pack.ID), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAddSticker_InvalidFileType(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "addsticker5", false)
	pack := createStickerPack(t, user.ID, "Pack")

	// TXT file (not an image)
	fileContent := []byte("text content")
	req, w := createMultipartStickerRequest(t, "POST", fmt.Sprintf("/api/v1/sticker_packs/%d/stickers", pack.ID), "sticker_file", "sticker.txt", fileContent, "smile", &user.ID)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestAddSticker_Success(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "addsticker6", false)
	pack := createStickerPack(t, user.ID, "Pack")

	// Valid PNG file
	fileContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	req, w := createMultipartStickerRequest(t, "POST", fmt.Sprintf("/api/v1/sticker_packs/%d/stickers", pack.ID), "sticker_file", "sticker.png", fileContent, "smile", &user.ID)
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
// DeleteSticker Tests
// ============================================================================

func TestDeleteSticker_Unauthorized(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "deletesticker1", false)
	pack := createStickerPack(t, user.ID, "Pack")
	sticker := createSticker(t, pack.ID, user.ID, "smile", "/stickers/smile.png")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/stickers/%d", sticker.ID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestDeleteSticker_NotFound(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "deletesticker2", false)

	req, _ := http.NewRequest("DELETE", "/api/v1/stickers/99999", nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDeleteSticker_AccessDenied(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user1 := createStickersUser(t, "deletesticker3a", false)
	user2 := createStickersUser(t, "deletesticker3b", false)
	pack := createStickerPack(t, user1.ID, "Pack")
	sticker := createSticker(t, pack.ID, user1.ID, "smile", "/stickers/smile.png")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/stickers/%d", sticker.ID), nil)
	addAuthCookieStickers(req, user2.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestDeleteSticker_Success(t *testing.T) {
	_, router, cleanup := setupStickersTestDB(t)
	defer cleanup()
	user := createStickersUser(t, "deletesticker4", false)
	pack := createStickerPack(t, user.ID, "Pack")
	sticker := createSticker(t, pack.ID, user.ID, "smile", "/stickers/smile.png")

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/stickers/%d", sticker.ID), nil)
	addAuthCookieStickers(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify sticker deleted
	var deletedSticker models.Sticker
	result := database.DB.First(&deletedSticker, sticker.ID)
	if result.Error == nil {
		t.Error("DeleteSticker() should delete the sticker")
	}
}
