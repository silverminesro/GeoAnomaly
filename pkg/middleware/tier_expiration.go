package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TierService interface to avoid import cycle
type TierService interface {
	CheckAndResetExpiredTier(userID uuid.UUID) error
}

// TierExpirationMiddleware checks and resets expired tiers for authenticated users
func TierExpirationMiddleware(tierService TierService) gin.HandlerFunc {
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
			log.Printf("⚠️  [TIER] Invalid user_id type in context: %T", userIDInterface)
			c.Next()
			return
		}

		// Check and reset expired tier
		log.Printf("🔍 [TIER] Checking tier expiration for user: %s", userID.String())
		if err := tierService.CheckAndResetExpiredTier(userID); err != nil {
			// Log error but don't block request
			log.Printf("❌ [TIER] Error checking tier for user %s: %v", userID.String(), err)
			c.Error(err)
		} else {
			log.Printf("✅ [TIER] Tier check completed for user: %s", userID.String())
		}

		c.Next()
	}
}
