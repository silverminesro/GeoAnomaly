package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func RateLimit(client *redis.Client) gin.HandlerFunc {
	limiter := &RateLimiter{
		client: client,
		limit:  100,         // 100 requests
		window: time.Minute, // per minute
	}

	return limiter.Limit()
}

func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()

		// Check if user is authenticated for higher limits
		if userID, exists := c.Get("user_id"); exists {
			clientIP = fmt.Sprintf("user:%s", userID)
			rl.limit = 300 // Higher limit for authenticated users
		}

		key := fmt.Sprintf("rate_limit:%s", clientIP)

		// Get current count
		val, err := rl.client.Get(context.Background(), key).Result()
		if err != nil && err != redis.Nil {
			// If Redis is down, allow the request
			c.Next()
			return
		}

		var currentCount int
		if err == redis.Nil {
			currentCount = 0
		} else {
			currentCount, _ = strconv.Atoi(val)
		}

		// Check if limit exceeded
		if currentCount >= rl.limit {
			c.Header("X-RateLimit-Limit", strconv.Itoa(rl.limit))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(rl.window).Unix(), 10))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(rl.window.Seconds()),
			})
			c.Abort()
			return
		}

		// Increment counter
		pipe := rl.client.Pipeline()
		pipe.Incr(context.Background(), key)
		pipe.Expire(context.Background(), key, rl.window)
		_, err = pipe.Exec(context.Background())

		if err != nil {
			// If Redis operation fails, log but continue
			fmt.Printf("Rate limiter error: %v\n", err)
		}

		// Set headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(rl.limit-currentCount-1))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(rl.window).Unix(), 10))

		c.Next()
	}
}
