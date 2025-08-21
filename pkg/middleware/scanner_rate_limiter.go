package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"geoanomaly/internal/scanner"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ScannerRateLimiter struct {
	client *redis.Client
	db     *gorm.DB
}

func NewScannerRateLimiter(client *redis.Client, db *gorm.DB) *ScannerRateLimiter {
	return &ScannerRateLimiter{
		client: client,
		db:     db,
	}
}

// ScannerRateLimit - rate limiting middleware pre scanner založený na scanner capabilities
func (srl *ScannerRateLimiter) ScannerRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Získaj user ID
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Získaj scanner inštanciu
		scannerService := scanner.NewService(srl.db)
		instance, err := scannerService.GetOrCreateScannerInstance(userID.(uuid.UUID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scanner instance"})
			c.Abort()
			return
		}

		// Vypočítať scanner stats
		stats, err := scannerService.CalculateScannerStats(instance)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate scanner stats"})
			c.Abort()
			return
		}

		// Vypočítať rate limit založený na scanner polling rate
		maxRequestsPerMinute := int(stats.ServerPollHz * 60) // Konvertuj Hz na requests za minútu
		
		// Minimum 60 requests za minútu, maximum 300
		if maxRequestsPerMinute < 60 {
			maxRequestsPerMinute = 60
		}
		if maxRequestsPerMinute > 300 {
			maxRequestsPerMinute = 300
		}

		// Skontroluj rate limit
		if !srl.checkScannerRateLimit(userID.(uuid.UUID).String(), maxRequestsPerMinute) {
			// Vypočítať čas do ďalšieho requestu
			nextRequestTime := srl.getNextRequestTime(userID.(uuid.UUID).String())
			waitTime := nextRequestTime.Sub(time.Now())

			c.Header("X-RateLimit-Limit", strconv.Itoa(maxRequestsPerMinute))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(nextRequestTime.Unix(), 10))
			c.Header("X-Scanner-PollHz", fmt.Sprintf("%.2f", stats.ServerPollHz))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Scanner rate limit exceeded",
				"details": gin.H{
					"scanner_poll_hz":     stats.ServerPollHz,
					"max_requests_per_min": maxRequestsPerMinute,
					"wait_time_seconds":   int(waitTime.Seconds()),
					"next_request_at":     nextRequestTime.Unix(),
				},
				"message": fmt.Sprintf("Scanner polling too fast. Wait %d seconds.", int(waitTime.Seconds())),
			})
			c.Abort()
			return
		}

		// Nastav headers
		remaining := srl.getRemainingRequests(userID.(uuid.UUID).String(), maxRequestsPerMinute)
		c.Header("X-RateLimit-Limit", strconv.Itoa(maxRequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-Scanner-PollHz", fmt.Sprintf("%.2f", stats.ServerPollHz))

		c.Next()
	}
}

// checkScannerRateLimit - skontroluje či hráč neprekročil rate limit
func (srl *ScannerRateLimiter) checkScannerRateLimit(userID string, maxRequests int) bool {
	if srl.client == nil {
		return true // Allow if Redis unavailable
	}

	key := fmt.Sprintf("scanner_rate:%s", userID)
	
	// Získaj aktuálny počet requestov
	val, err := srl.client.Get(context.Background(), key).Result()
	if err != nil && err != redis.Nil {
		return true // Allow if Redis error
	}

	var currentCount int
	if err == redis.Nil {
		currentCount = 0
	} else {
		currentCount, _ = strconv.Atoi(val)
	}

	// Skontroluj či limit prekročený
	if currentCount >= maxRequests {
		return false
	}

	// Zvýš počítadlo
	pipe := srl.client.Pipeline()
	pipe.Incr(context.Background(), key)
	pipe.Expire(context.Background(), key, time.Minute)
	_, err = pipe.Exec(context.Background())

	if err != nil {
		// Log error ale pokračuj
		fmt.Printf("Scanner rate limiter error: %v\n", err)
	}

	return true
}

// getRemainingRequests - vráti počet zostávajúcich requestov
func (srl *ScannerRateLimiter) getRemainingRequests(userID string, maxRequests int) int {
	if srl.client == nil {
		return maxRequests
	}

	key := fmt.Sprintf("scanner_rate:%s", userID)
	val, err := srl.client.Get(context.Background(), key).Result()
	if err != nil && err != redis.Nil {
		return maxRequests
	}

	var currentCount int
	if err == redis.Nil {
		currentCount = 0
	} else {
		currentCount, _ = strconv.Atoi(val)
	}

	remaining := maxRequests - currentCount
	if remaining < 0 {
		remaining = 0
	}

	return remaining
}

// getNextRequestTime - vráti čas ďalšieho možného requestu
func (srl *ScannerRateLimiter) getNextRequestTime(userID string) time.Time {
	if srl.client == nil {
		return time.Now()
	}

	key := fmt.Sprintf("scanner_rate:%s", userID)
	ttl, err := srl.client.TTL(context.Background(), key).Result()
	if err != nil {
		return time.Now().Add(time.Minute)
	}

	return time.Now().Add(ttl)
}
