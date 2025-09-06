package auth

import (
	"context"
	"fmt"
	"geoanomaly/pkg/middleware"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Handler struct {
	db    *gorm.DB
	redis *redis.Client
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

func NewHandler(db *gorm.DB, redis *redis.Client) *Handler {
	return &Handler{
		db:    db,
		redis: redis,
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Check if user already exists
	var existingUser User
	if err := h.db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		if existingUser.Username == req.Username {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		if existingUser.Email == req.Email {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user with tier 1 (basic free tier)
	user := User{
		BaseModel: BaseModel{
			ID: uuid.New(),
		},
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Tier:         1, // Tier 1 = základný free tier
		IsActive:     true,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create user",
			"details": err.Error(),
		})
		return
	}

	// Generate JWT token
	token, err := middleware.GenerateJWT(user.ID, user.Username, user.Tier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Remove password hash from response
	user.PasswordHash = ""

	c.JSON(http.StatusCreated, gin.H{
		"message":   "User registered successfully",
		"token":     token,
		"user":      user,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Find user by username or email
	var user User
	if err := h.db.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid credentials",
			"message": "Username/email or password is incorrect",
		})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid credentials",
			"message": "Username/email or password is incorrect",
		})
		return
	}

	// Check if user is active
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Account is deactivated",
			"message": "Your account has been deactivated. Please contact support.",
		})
		return
	}

	// Update last login timestamp
	now := time.Now()
	h.db.Model(&user).Updates(map[string]interface{}{
		"updated_at": now,
		// "last_login": now, // Uncomment if you add this field to User model
	})

	// Generate JWT token
	token, err := middleware.GenerateJWT(user.ID, user.Username, user.Tier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Remove password hash from response
	user.PasswordHash = ""

	c.JSON(http.StatusOK, gin.H{
		"message":   "Login successful",
		"token":     token,
		"user":      user,
		"expires":   time.Now().Add(24 * time.Hour).Unix(),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) RefreshToken(c *gin.Context) {
	// Get user from context (set by JWT middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	// ✅ OPRAVENÉ: Removed unused variables username and tier
	// We get fresh data from database instead

	// Verify user still exists and is active
	var user User
	if err := h.db.Where("id = ? AND is_active = true", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "User not found or inactive",
			"message": "Your account may have been deactivated",
		})
		return
	}

	// Use current user data from DB (in case tier changed)
	token, err := middleware.GenerateJWT(user.ID, user.Username, user.Tier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Token refreshed successfully",
		"token":     token,
		"expires":   time.Now().Add(24 * time.Hour).Unix(),
		"user_tier": user.Tier,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Logout endpoint - improved implementation
func (h *Handler) Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	// If using Redis for session management, invalidate token here
	if h.redis != nil {
		tokenKey := fmt.Sprintf("blacklist:token:%s", userID)
		h.redis.Set(context.Background(), tokenKey, "blacklisted", 24*time.Hour)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Logged out successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"note":      "Please delete the token from your client",
	})
}

// GetProfile endpoint for getting current user info
func (h *Handler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	var user User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Remove password hash from response
	user.PasswordHash = ""

	c.JSON(http.StatusOK, gin.H{
		"user":      user,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ValidateToken endpoint for debugging
func (h *Handler) ValidateToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	tier, _ := c.Get("tier")

	c.JSON(http.StatusOK, gin.H{
		"valid":     true,
		"user_id":   userID,
		"username":  username,
		"tier":      tier,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "Token is valid",
	})
}

// ChangePassword endpoint
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	type ChangePasswordRequest struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Get user from database
	var user User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Update password
	if err := h.db.Model(&user).Update("password_hash", string(hashedPassword)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Password changed successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
