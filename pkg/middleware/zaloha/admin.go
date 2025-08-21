package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminOrHigher middleware - requires tier >= 4 (Admin or Super Admin)
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		tier, exists := c.Get("tier")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Authentication required",
				"message": "Please login first",
			})
			c.Abort()
			return
		}

		username, _ := c.Get("username")
		userTier, ok := tier.(int)
		if !ok || userTier < 4 {
			c.JSON(http.StatusForbidden, gin.H{
				"error":         "Admin access required",
				"message":       "You need Admin (Tier 4+) privileges",
				"required_tier": 4,
				"your_tier":     userTier,
				"current_user":  username,
			})
			c.Abort()
			return
		}

		// Log admin access
		adminLevel := "Admin"
		if userTier == 5 {
			adminLevel = "Super Admin"
		}
		log.Printf("🔧 %s ACCESS: %s (Tier %d) → %s %s",
			adminLevel, username, userTier, c.Request.Method, c.Request.URL.Path)

		c.Set("admin_level", adminLevel)
		c.Next()
	}
}

// SuperAdminOnly middleware - requires tier = 5 (silverminesro + K4RDAN only)
func SuperAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		tier, exists := c.Get("tier")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		username, _ := c.Get("username")
		userTier, ok := tier.(int)
		if !ok || userTier < 5 {
			c.JSON(http.StatusForbidden, gin.H{
				"error":         "Super Admin access required",
				"message":       "Only silverminesro and K4RDAN have Super Admin access",
				"required_tier": 5,
				"your_tier":     userTier,
				"current_user":  username,
			})
			c.Abort()
			return
		}

		// Log Super Admin access
		log.Printf("👑 SUPER ADMIN ACCESS: %s (Tier %d) → %s %s",
			username, userTier, c.Request.Method, c.Request.URL.Path)

		c.Next()
	}
}

// LegendaryOrHigher middleware - requires tier >= 3
func LegendaryOrHigher() gin.HandlerFunc {
	return func(c *gin.Context) {
		tier, exists := c.Get("tier")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		userTier, ok := tier.(int)
		if !ok || userTier < 3 {
			c.JSON(http.StatusForbidden, gin.H{
				"error":         "Legendary User access required",
				"message":       "Upgrade to Legendary (Tier 3+) for exclusive features",
				"required_tier": 3,
				"your_tier":     userTier,
				"upgrade_url":   "/upgrade/legendary",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PremiumOrHigher middleware - requires tier >= 2
func PremiumOrHigher() gin.HandlerFunc {
	return func(c *gin.Context) {
		tier, exists := c.Get("tier")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}

		userTier, ok := tier.(int)
		if !ok || userTier < 2 {
			c.JSON(http.StatusForbidden, gin.H{
				"error":         "Premium access required",
				"message":       "Upgrade to Premium (Tier 2+) for enhanced features",
				"required_tier": 2,
				"your_tier":     userTier,
				"upgrade_url":   "/upgrade/premium",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Legacy alias for compatibility
func ModeratorOnly() gin.HandlerFunc {
	return LegendaryOrHigher()
}
