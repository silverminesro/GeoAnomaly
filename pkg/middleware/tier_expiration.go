package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"geoanomaly/internal/menu"
)

// TierExpirationMiddleware checks and resets expired tiers for authenticated users
func TierExpirationMiddleware(menuService *menu.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by JWT middleware)
		userIDInterface, exists := c.Get("user_id")
		if !exists {
			// User not authenticated, continue
			c.Next()
			return
		}

		userID, ok := userIDInterface.(uuid.UUID)
		if !ok {
			// Invalid user ID, continue
			c.Next()
			return
		}

		// Check and reset expired tier
		if err := menuService.CheckAndResetExpiredTier(userID); err != nil {
			// Log error but don't block request
			c.Error(err)
		}

		c.Next()
	}
}
