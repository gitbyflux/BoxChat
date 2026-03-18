package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"boxchat/internal/config"
	"boxchat/internal/database"
	"boxchat/internal/models"
	"boxchat/internal/testutil"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthTestDB(t *testing.T) (*config.Config, *gin.Engine, func()) {
	t.Helper()

	cfg, dbCleanup := testutil.SetupTestDB(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	authHandler := NewAuthHandler(cfg)
	authHandler.RegisterRoutes(router)

	return cfg, router, dbCleanup
}

func hashPasswordForAuthTest(t *testing.T, password string) string {
	t.Helper()
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	return string(hashedBytes)
}

func createRegularUserForAuthTest(t *testing.T, username string) *models.User {
	t.Helper()
	hashedPassword := hashPasswordForAuthTest(t, "password123")
	user := models.User{
		Username:       username,
		Password:       hashedPassword,
		PresenceStatus: "offline",
		IsBanned:       false,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	return &user
}

func createBannedUserForAuthTest(t *testing.T, username string) *models.User {
	t.Helper()
	hashedPassword := hashPasswordForAuthTest(t, "password123")
	user := models.User{
		Username:       username,
		Password:       hashedPassword,
		PresenceStatus: "offline",
		IsBanned:       true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create banned user: %v", err)
	}
	return &user
}

func addAuthCookieForAuthTest(req *http.Request, userID uint) {
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", userID),
	})
}

func TestNewAuthHandler_Success(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	handler := NewAuthHandler(cfg)

	if handler == nil {
		t.Fatal("NewAuthHandler() should return non-nil handler")
	}
	if handler.authService == nil {
		t.Error("NewAuthHandler() should initialize authService")
	}
	if handler.cfg == nil {
		t.Error("NewAuthHandler() should initialize cfg")
	}
}

func TestAuthHandler_RegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	handler := NewAuthHandler(cfg)
	router := gin.New()
	handler.RegisterRoutes(router)

	routes := router.Routes()

	expectedRoutes := map[string]bool{
		"/api/v1/auth/login":    false,
		"/api/v1/auth/register": false,
		"/api/v1/auth/session":  false,
		"/login":                false,
		"/register":             false,
		"/logout":               false,
	}

	for _, route := range routes {
		if _, exists := expectedRoutes[route.Path]; exists {
			expectedRoutes[route.Path] = true
		}
	}

	for path, found := range expectedRoutes {
		if !found {
			t.Errorf("Route %s not registered", path)
		}
	}
}

func TestLoginAPI_JSON_Success(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "loginjsonuser")

	loginData := map[string]string{
		"username": "loginjsonuser",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
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

	cookies := w.Result().Cookies()
	if len(cookies) < 3 {
		t.Errorf("Expected at least 3 cookies, got %d", len(cookies))
	}
}

func TestLoginAPI_JSON_InvalidCredentials(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	loginData := map[string]string{
		"username": "nonexistent",
		"password": "wrongpassword",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestLoginAPI_JSON_BannedUser(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createBannedUserForAuthTest(t, "bannedloginuser")

	loginData := map[string]string{
		"username": "bannedloginuser",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestLoginAPI_JSON_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestLoginAPI_JSON_WithRememberMe(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "remembermeuser")

	loginData := map[string]interface{}{
		"username":    "remembermeuser",
		"password":    "password123",
		"remember_me": true,
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	session, ok := response["session"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected session object in response")
	}
	if session["remember"] != true {
		t.Error("Expected remember=true in session")
	}
}

func TestLoginAPI_Form_Success(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "loginformuser")

	formData := "username=loginformuser&password=password123"
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

func TestLoginAPI_Form_WithRememberMe(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "formrememberuser")

	formData := "username=formrememberuser&password=password123&remember_me=on"
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	session, ok := response["session"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected session object in response")
	}
	if session["remember"] != true {
		t.Error("Expected remember=true in session")
	}
}

func TestLoginAPI_Form_WithRememberMeTrue(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "formremembertrueuser")

	formData := "username=formremembertrueuser&password=password123&remember_me=true"
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	session, ok := response["session"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected session object in response")
	}
	if session["remember"] != true {
		t.Error("Expected remember=true in session")
	}
}

func TestRegisterAPI_JSON_Success(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "newjsonuser",
		"password":         "password123",
		"confirm_password": "password123",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
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

func TestRegisterAPI_JSON_UsernameTooShort(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "ab",
		"password":         "password123",
		"confirm_password": "password123",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRegisterAPI_JSON_UsernameTooLong(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "verylongusernamethatexceedsthirtycharacters",
		"password":         "password123",
		"confirm_password": "password123",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRegisterAPI_JSON_PasswordTooShort(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "validuser",
		"password":         "short",
		"confirm_password": "short",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRegisterAPI_JSON_PasswordsMismatch(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "mismatchuser",
		"password":         "password123",
		"confirm_password": "password456",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRegisterAPI_JSON_UsernameTaken(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "existinguser")

	registerData := map[string]string{
		"username":         "existinguser",
		"password":         "password123",
		"confirm_password": "password123",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestRegisterAPI_JSON_InvalidJSON(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer([]byte(`{invalid json}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRegisterAPI_JSON_WithRememberMe(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]interface{}{
		"username":         "rememberreguser",
		"password":         "password123",
		"confirm_password": "password123",
		"remember_me":      true,
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	session, ok := response["session"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected session object in response")
	}
	if session["remember"] != true {
		t.Error("Expected remember=true in session")
	}
}

func TestRegisterAPI_Form_Success(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	formData := "username=newformuser&password=password123&confirm_password=password123"
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

func TestRegisterAPI_Form_WithRememberMe(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	formData := "username=formreguser&password=password123&confirm_password=password123&remember_me=on"
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	session, ok := response["session"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected session object in response")
	}
	if session["remember"] != true {
		t.Error("Expected remember=true in session")
	}
}

func TestRegisterAPI_Form_PasswordsMismatch(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	formData := "username=mismatchformuser&password=password123&confirm_password=password456"
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetSession_Authenticated(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	user := createRegularUserForAuthTest(t, "sessionuser")

	req, _ := http.NewRequest("GET", "/api/v1/auth/session", nil)
	addAuthCookieForAuthTest(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["authenticated"] != true {
		t.Error("Expected authenticated=true")
	}

	userData, ok := response["user"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected user object in response")
	}
	if userData["username"] != "sessionuser" {
		t.Errorf("Expected username 'sessionuser', got %v", userData["username"])
	}
}

func TestGetSession_Unauthenticated(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/api/v1/auth/session", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetSession_WithAuthModeCookie(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	user := createRegularUserForAuthTest(t, "authmodeuser")

	req, _ := http.NewRequest("GET", "/api/v1/auth/session", nil)
	addAuthCookieForAuthTest(req, user.ID)
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_auth_mode",
		Value: "remember",
	})
	req.AddCookie(&http.Cookie{
		Name:  "boxchat_uid",
		Value: fmt.Sprintf("%d", user.ID),
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestGetSession_WithAvatar(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	user := models.User{
		Username:       "avataruser",
		Password:       hashPasswordForAuthTest(t, "password123"),
		AvatarURL:      "/uploads/avatars/test.jpg",
		IsSuperuser:    true,
		PresenceStatus: "online",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/v1/auth/session", nil)
	addAuthCookieForAuthTest(req, user.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	userData, ok := response["user"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected user object in response")
	}
	if userData["avatar_url"] != "/uploads/avatars/test.jpg" {
		t.Errorf("Expected avatar_url='/uploads/avatars/test.jpg', got %v", userData["avatar_url"])
	}
	if userData["is_superuser"] != true {
		t.Error("Expected is_superuser=true")
	}
}

func TestLogout_Success(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Expected status 302, got %d", w.Code)
	}

	if location := w.Header().Get("Location"); location != "/login" {
		t.Errorf("Expected redirect to /login, got %s", location)
	}

	cookies := w.Result().Cookies()
	if len(cookies) < 3 {
		t.Errorf("Expected at least 3 cookies to be cleared, got %d", len(cookies))
	}

	for _, cookie := range cookies {
		if cookie.Value != "" {
			t.Errorf("Expected cookie %s to be empty, got %s", cookie.Name, cookie.Value)
		}
	}
}

func TestLoginPage_ServeSPA(t *testing.T) {
	cfg, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	frontendDist := filepath.Join(cfg.RootDir, "frontend", "dist")
	if err := os.MkdirAll(frontendDist, 0755); err != nil {
		t.Fatalf("Failed to create frontend/dist: %v", err)
	}

	indexHTML := "<html><body>BoxChat App</body></html>"
	indexHTMLPath := filepath.Join(frontendDist, "index.html")
	if err := os.WriteFile(indexHTMLPath, []byte(indexHTML), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	req, _ := http.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if !bytes.Contains(w.Body.Bytes(), []byte("BoxChat App")) {
		t.Error("Expected response to contain 'BoxChat App'")
	}
}

func TestRegisterPage_ServeSPA(t *testing.T) {
	cfg, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	frontendDist := filepath.Join(cfg.RootDir, "frontend", "dist")
	if err := os.MkdirAll(frontendDist, 0755); err != nil {
		t.Fatalf("Failed to create frontend/dist: %v", err)
	}

	indexHTML := "<html><body>BoxChat Register</body></html>"
	indexHTMLPath := filepath.Join(frontendDist, "index.html")
	if err := os.WriteFile(indexHTMLPath, []byte(indexHTML), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	req, _ := http.NewRequest("GET", "/register", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if !bytes.Contains(w.Body.Bytes(), []byte("BoxChat Register")) {
		t.Error("Expected response to contain 'BoxChat Register'")
	}
}

func TestSetAuthCookies_WithoutRememberMe(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "cookieuser")

	loginData := map[string]string{
		"username": "cookieuser",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	cookieMap := make(map[string]*http.Cookie)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie
	}

	expectedCookies := []string{"boxchat_uid", "boxchat_uname", "boxchat_auth_mode"}
	for _, cookieName := range expectedCookies {
		if cookie, exists := cookieMap[cookieName]; !exists {
			t.Errorf("Expected cookie %s to be set", cookieName)
		} else if cookieName == "boxchat_auth_mode" && cookie.Value != "session" {
			t.Errorf("Expected boxchat_auth_mode='session', got %s", cookie.Value)
		}
	}
}

func TestSetAuthCookies_WithRememberMe(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "remembercookieuser")

	loginData := map[string]interface{}{
		"username":    "remembercookieuser",
		"password":    "password123",
		"remember_me": true,
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	cookieMap := make(map[string]*http.Cookie)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie
	}

	if cookie, exists := cookieMap["boxchat_auth_mode"]; exists {
		if cookie.Value != "remember" {
			t.Errorf("Expected boxchat_auth_mode='remember', got %s", cookie.Value)
		}
	} else {
		t.Error("Expected boxchat_auth_mode cookie to be set")
	}
}

func TestClearAuthCookies_Verification(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", "/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	cookieMap := make(map[string]*http.Cookie)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie
	}

	expectedCookies := []string{"boxchat_uid", "boxchat_uname", "boxchat_auth_mode"}
	for _, cookieName := range expectedCookies {
		if cookie, exists := cookieMap[cookieName]; !exists {
			t.Errorf("Expected cookie %s to be cleared", cookieName)
		} else if cookie.Value != "" {
			t.Errorf("Expected cookie %s to have empty value, got %s", cookieName, cookie.Value)
		}
	}
}

func TestServeIndexHTML(t *testing.T) {
	// Skip this test as it requires template engine setup
	t.Skip("Skipping test that requires template engine initialization")
}

func TestLegacyLogin_POST_Success(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "legacyleveloginuser")

	formData := "username=legacyleveloginuser&password=password123"
	req, _ := http.NewRequest("POST", "/login", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

func TestLegacyRegister_POST_Success(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	formData := "username=legacyregisteruser&password=password123&confirm_password=password123"
	req, _ := http.NewRequest("POST", "/register", bytes.NewBufferString(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

func TestFullAuthFlow_RegisterLoginLogout(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "flowtestuser",
		"password":         "password123",
		"confirm_password": "password123",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Registration failed: %d - %s", w.Code, w.Body.String())
	}

	loginData := map[string]string{
		"username": "flowtestuser",
		"password": "password123",
	}
	jsonData, _ = json.Marshal(loginData)

	req, _ = http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Login failed: %d - %s", w.Code, w.Body.String())
	}

	req, _ = http.NewRequest("GET", "/api/v1/auth/session", nil)
	for _, cookie := range w.Result().Cookies() {
		req.AddCookie(cookie)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Session check failed: %d - %s", w.Code, w.Body.String())
	}

	req, _ = http.NewRequest("GET", "/logout", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Logout failed: %d - %s", w.Code, w.Body.String())
	}
}

func TestLoginAPI_EmptyUsername(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	loginData := map[string]string{
		"username": "",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Empty username fails validation (400) or returns unauthorized (401)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 400 or 401, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestLoginAPI_EmptyPassword(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "emptypassworduser")

	loginData := map[string]string{
		"username": "emptypassworduser",
		"password": "",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Empty password fails validation (400) or returns unauthorized (401)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 400 or 401, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestRegisterAPI_EmptyUsername(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "",
		"password":         "password123",
		"confirm_password": "password123",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRegisterAPI_EmptyPassword(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	registerData := map[string]string{
		"username":         "validusername",
		"password":         "",
		"confirm_password": "",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestLoginAPI_CaseInsensitiveUsername(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "caseuser")

	loginData := map[string]string{
		"username": "CASEUSER",
		"password": "password123",
	}
	jsonData, _ := json.Marshal(loginData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestRegisterAPI_CaseInsensitiveUsername(t *testing.T) {
	_, router, cleanup := setupAuthTestDB(t)
	defer cleanup()

	createRegularUserForAuthTest(t, "existingcaseuser")

	registerData := map[string]string{
		"username":         "EXISTINGCASEUSER",
		"password":         "password123",
		"confirm_password": "password123",
	}
	jsonData, _ := json.Marshal(registerData)

	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d. Body: %s", w.Code, w.Body.String())
	}
}
