package http

import (
	"net/http"
	"path/filepath"
	"strconv"
	"boxchat/internal/config"
	"boxchat/internal/middleware"
	"boxchat/internal/models"
	"boxchat/internal/services"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *services.AuthService
	cfg         *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService: services.NewAuthService(),
		cfg:         cfg,
	}
}

func (h *AuthHandler) RegisterRoutes(r *gin.Engine) {
	auth := r.Group("/")
	{
		auth.POST("/api/v1/auth/login", h.LoginAPI)
		auth.POST("/api/v1/auth/register", h.RegisterAPI)
		auth.GET("/api/v1/auth/session", middleware.Auth(), h.GetSession)

		// Legacy routes for SPA
		auth.GET("/login", h.LoginPage)
		auth.GET("/register", h.RegisterPage)
		auth.POST("/login", h.LoginAPI)
		auth.POST("/register", h.RegisterAPI)
		auth.GET("/logout", h.Logout)
	}
}

func (h *AuthHandler) LoginAPI(c *gin.Context) {
	var req services.LoginRequest
	
	if c.Request.Header.Get("Content-Type") == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	} else {
		req.Username = c.PostForm("username")
		req.Password = c.PostForm("password")
		req.RememberMe = c.PostForm("remember_me") == "on" || c.PostForm("remember_me") == "true"
	}
	
	user, err := h.authService.Login(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": err.Error()})
		return
	}
	
	// Set cookies
	h.setAuthCookies(c, user, req.RememberMe)
	
	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"redirect": "/",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
		"session": gin.H{
			"remember":    req.RememberMe,
			"cookie_name": h.cfg.SessionCookieName,
		},
	})
}

func (h *AuthHandler) RegisterAPI(c *gin.Context) {
	var req services.RegisterRequest
	
	if c.Request.Header.Get("Content-Type") == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	} else {
		req.Username = c.PostForm("username")
		req.Password = c.PostForm("password")
		req.ConfirmPassword = c.PostForm("confirm_password")
		req.RememberMe = c.PostForm("remember_me") == "on" || c.PostForm("remember_me") == "true"
	}
	
	user, err := h.authService.Register(&req)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "username already taken" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"success": false, "error": err.Error()})
		return
	}
	
	// Set cookies
	h.setAuthCookies(c, user, req.RememberMe)
	
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"redirect": "/",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
		"session": gin.H{
			"remember":    req.RememberMe,
			"cookie_name": h.cfg.SessionCookieName,
		},
	})
}

func (h *AuthHandler) GetSession(c *gin.Context) {
	user, exists := middleware.GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
		return
	}
	
	authMode, _ := c.Cookie("boxchat_auth_mode")
	uidCookie, _ := c.Cookie("boxchat_uid")
	
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user": gin.H{
			"id":           user.ID,
			"username":     user.Username,
			"avatar_url":   user.AvatarURL,
			"is_superuser": user.IsSuperuser,
		},
		"session": gin.H{
			"cookie_name":        h.cfg.SessionCookieName,
			"remember_cookie_name": h.cfg.RememberCookieName,
			"auth_mode":          authMode,
			"uid_cookie":         uidCookie,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Clear session cookies
	h.clearAuthCookies(c)
	c.Redirect(http.StatusFound, "/login")
}

func (h *AuthHandler) LoginPage(c *gin.Context) {
	// Serve SPA index.html
	frontendDist := filepath.Join(h.cfg.RootDir, "frontend", "dist")
	http.ServeFile(c.Writer, c.Request, filepath.Join(frontendDist, "index.html"))
}

func (h *AuthHandler) RegisterPage(c *gin.Context) {
	// Serve SPA index.html
	frontendDist := filepath.Join(h.cfg.RootDir, "frontend", "dist")
	http.ServeFile(c.Writer, c.Request, filepath.Join(frontendDist, "index.html"))
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, user *models.User, remember bool) {
	maxAge := int(h.cfg.RememberCookieDuration.Seconds())
	if !remember {
		maxAge = 0
	}

	secure := h.cfg.SessionCookieSecure
	httpOnly := true

	// Set cookies with SameSite=Lax for browser compatibility
	// Use empty domain ("") for host-only cookies - works on any host
	// Path "/" makes cookies available to all paths
	c.SetCookie("boxchat_uid", strconv.FormatUint(uint64(user.ID), 10), maxAge, "/", "", secure, httpOnly)
	c.SetCookie("boxchat_uname", user.Username, maxAge, "/", "", secure, httpOnly)

	authMode := "session"
	if remember {
		authMode = "remember"
	}
	c.SetCookie("boxchat_auth_mode", authMode, maxAge, "/", "", secure, httpOnly)
}

func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	httpOnly := true
	c.SetCookie("boxchat_uid", "", -1, "/", "", false, httpOnly)
	c.SetCookie("boxchat_uname", "", -1, "/", "", false, httpOnly)
	c.SetCookie("boxchat_auth_mode", "", -1, "/", "", false, httpOnly)
}

// Dummy handler for static files
func ServeIndexHTML(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}
