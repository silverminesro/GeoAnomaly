package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func getEnvVar(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Allow specific origins (add your Flutter app domains)
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:8080",  // Flutter web default
			"http://localhost:5000",  // Flutter web alternative
			"https://yourdomain.com",
			"capacitor://localhost",
			"ionic://localhost",
		}

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// For development, allow all origins if configured
		if !allowed && getEnvVar("CORS_ALLOW_ALL", "false") == "true" {
			allowed = true
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
