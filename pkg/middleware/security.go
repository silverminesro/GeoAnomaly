package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type SecurityMiddleware struct {
	client *redis.Client
}

// 🟢 WHITELIST: Development/Admin IPs (never blacklist these)
var whitelistedIPs = map[string]bool{
	"91.127.107.191": true, // silverminesro - development IP
	"127.0.0.1":      true, // localhost
	"::1":            true, // localhost IPv6
	"localhost":      true, // localhost domain
}

// Blacklisted IPs from recent attacks
var blacklistedIPs = map[string]bool{
	"35.193.149.100": true,
	"185.91.127.107": true,
	"185.169.4.150":  true,
	"204.76.203.193": true,
	"80.75.212.17":   true,
	"207.167.67.206": true,
	"36.41.184.119":  true,
	"65.49.1.108":    true,
	"200.81.185.179": true,
	"104.234.115.61": true,
	"20.15.201.69":   true,
	"35.216.241.81":  true,
	"103.93.93.182":  true,
	"176.65.148.178": true,
	"196.251.69.116": true,
}

// 🛡️ PLAYER-FRIENDLY: Iba skutočne nebezpečné cesty
var suspiciousPaths = []string{
	"/boaform/", "/.env", "/wp-admin/", "/.git/",
	"/phpmyadmin/", "/xmlrpc.php", "/wp-content/",
	"/cgi-bin/", "/config.php", "/wp-config.php",
	"/.htaccess", "/shell", "/webshell", "/backdoor",
	"/exploit", "/sqlmap", "/nuclei", "/nmap",
	// 🚫 ODSTRÁNENÉ všetky legitímne cesty ako /admin/, /config, /vendor/, atď.
}

// 🟢 LEGITÍMNE user-agenty (Flutter, mobile apps, browsers)
var legitimateUserAgents = []string{
	"flutter", "dart", "okhttp", "volley", "alamofire",
	"mozilla", "chrome", "firefox", "safari", "edge",
	"android", "ios", "mobile", "capacitor", "ionic",
	"react-native", "cordova", "electron",
}

// 🛡️ HLAVNÁ FUNKCIA: Security middleware s Redis
func Security(client *redis.Client) gin.HandlerFunc {
	sm := &SecurityMiddleware{client: client}
	return sm.securityCheck()
}

// 🛡️ PLAYER-FRIENDLY: Basic security bez Redis
func BasicSecurity() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		userAgent := c.Request.UserAgent()

		// 🟢 Whitelist check - NIKDY neblokuj whitelisted IPs
		if whitelistedIPs[clientIP] {
			log.Printf("🟢 [SECURITY] WHITELISTED IP allowed: %s %s %s", clientIP, method, path)
			c.Next()
			return
		}

		// 🟢 Legitimate user-agent check
		if isLegitimateUserAgent(userAgent) {
			if !isHealthCheckPath(path) {
				log.Printf("🟢 [SECURITY] LEGITIMATE APP: %s %s %s (UA: %s)", clientIP, method, path, userAgent[:min(50, len(userAgent))])
			}
			c.Next()
			return
		}

		// 1. 🚨 CONNECT method blocking (najdôležitejšie!)
		if method == "CONNECT" {
			log.Printf("🚨 [SECURITY] CONNECT ATTACK from: %s - BLOCKED", clientIP)
			blacklistedIPs[clientIP] = true
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"error": "Method not allowed",
				"code":  "CONNECT_BLOCKED",
			})
			c.Abort()
			return
		}

		// 2. 🚫 Blacklist check
		if blacklistedIPs[clientIP] {
			log.Printf("🚫 [SECURITY] BLOCKED blacklisted IP: %s %s %s", clientIP, method, path)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
				"code":  "IP_BLACKLISTED",
			})
			c.Abort()
			return
		}

		// 3. ⚠️ STRICT: Iba skutočne nebezpečné cesty
		for _, suspPath := range suspiciousPaths {
			if strings.Contains(path, suspPath) {
				log.Printf("⚠️ [SECURITY] ATTACK PATH from %s: %s %s", clientIP, method, path)
				// Okamžité blokovanie pre kritické legacy cesty
				if suspPath == "/cgi-bin/" || suspPath == "/boaform/" {
					blacklistedIPs[clientIP] = true
					log.Printf("🚫 [SECURITY] IMMEDIATE BLACKLIST: %s (critical attack path)", clientIP)
				} else {
					blacklistedIPs[clientIP] = true
				}
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Not found",
				})
				c.Abort()
				return
			}
		}

		// 4. 🤖 STRICT bot detection - iba skutočne nebezpečné boty
		if isAttackBot(userAgent) {
			log.Printf("🤖 [SECURITY] ATTACK BOT from %s: %s", clientIP, userAgent)
			blacklistedIPs[clientIP] = true
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
				"code":  "BOT_BLOCKED",
			})
			c.Abort()
			return
		}

		// 5. ✅ Allow all other traffic (development friendly)
		if !isHealthCheckPath(path) {
			log.Printf("✅ [SECURITY] ALLOWED: %s %s %s", clientIP, method, path)
		}

		c.Next()
	}
}

// 🛡️ REDIS SECURITY CHECK (plná funkcionalita)
func (sm *SecurityMiddleware) securityCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		userAgent := c.Request.UserAgent()

		// 🟢 Whitelist check - NIKDY neblokuj whitelisted IPs
		if whitelistedIPs[clientIP] {
			if !isHealthCheckPath(path) {
				log.Printf("🟢 [SECURITY] WHITELISTED IP allowed: %s %s %s", clientIP, method, path)
			}
			c.Next()
			return
		}

		// 🟢 Legitimate user-agent check (Flutter, mobile apps)
		if isLegitimateUserAgent(userAgent) {
			if !isHealthCheckPath(path) {
				log.Printf("🟢 [SECURITY] LEGITIMATE APP: %s %s %s", clientIP, method, path)
			}
			c.Next()
			return
		}

		// 1. 🚫 Blacklist check
		if blacklistedIPs[clientIP] {
			log.Printf("🚫 [SECURITY] BLOCKED blacklisted IP: %s %s %s", clientIP, method, path)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
				"code":  "IP_BLACKLISTED",
			})
			c.Abort()
			return
		}

		// 2. 🚨 CONNECT method attack (KRITICKÉ!)
		if method == "CONNECT" {
			log.Printf("🚨 [SECURITY] CONNECT ATTACK from: %s - AUTO BLACKLISTING", clientIP)
			blacklistedIPs[clientIP] = true
			sm.saveToRedisBlacklist(clientIP, "CONNECT_ATTACK")
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"error": "Method not allowed",
				"code":  "CONNECT_BLOCKED",
			})
			c.Abort()
			return
		}

		// 3. ⚠️ STRICT: Iba skutočne nebezpečné cesty
		for _, suspPath := range suspiciousPaths {
			if strings.Contains(path, suspPath) {
				log.Printf("⚠️ [SECURITY] ATTACK PATH from %s: %s %s", clientIP, method, path)
				// Okamžité blokovanie pre kritické legacy cesty
				if suspPath == "/cgi-bin/" || suspPath == "/boaform/" {
					blacklistedIPs[clientIP] = true
					sm.saveToRedisBlacklist(clientIP, "CRITICAL_ATTACK_PATH")
					log.Printf("🚫 [SECURITY] IMMEDIATE BLACKLIST: %s (critical attack path)", clientIP)
				} else {
					suspCount := sm.incrementSuspiciousCount(clientIP)
					if suspCount >= 5 {
						log.Printf("🚫 [SECURITY] AUTO-BLACKLISTED: %s (5+ attack attempts)", clientIP)
						blacklistedIPs[clientIP] = true
						sm.saveToRedisBlacklist(clientIP, "ATTACK_PATHS")
					}
				}
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Not found",
				})
				c.Abort()
				return
			}
		}

		// 4. 🤖 STRICT bot detection - iba attack boty
		if isAttackBot(userAgent) {
			log.Printf("🤖 [SECURITY] ATTACK BOT from %s: %s", clientIP, userAgent)

			botCount := sm.incrementBotCount(clientIP)
			if botCount >= 3 { // Okamžité blokovanie attack botov
				log.Printf("🚫 [SECURITY] AUTO-BLACKLISTED: %s (attack bot)", clientIP)
				blacklistedIPs[clientIP] = true
				sm.saveToRedisBlacklist(clientIP, "ATTACK_BOT")
			}

			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
				"code":  "BOT_BLOCKED",
			})
			c.Abort()
			return
		}

		// 5. 📊 RELAXED rate limiting pre neautentifikovaných
		if _, exists := c.Get("user_id"); !exists {
			if !sm.checkUnauthenticatedRateLimit(clientIP) {
				log.Printf("🚫 [SECURITY] RATE LIMIT: %s", clientIP)
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":       "Too many requests",
					"message":     "Please slow down or authenticate",
					"retry_after": 60,
				})
				c.Abort()
				return
			}
		}

		// 6. ✅ Allow all other traffic
		if !isHealthCheckPath(path) {
			log.Printf("✅ [SECURITY] ALLOWED: %s %s %s", clientIP, method, path)
		}

		c.Next()
	}
}

// 🟢 NOVÉ: Check if user-agent is from legitimate app
func isLegitimateUserAgent(userAgent string) bool {
	if userAgent == "" {
		return false
	}

	userAgentLower := strings.ToLower(userAgent)

	for _, legitimate := range legitimateUserAgents {
		if strings.Contains(userAgentLower, legitimate) {
			return true
		}
	}

	return false
}

// 🛡️ STRICT: Iba skutočne útočné boty
func isAttackBot(userAgent string) bool {
	if userAgent == "" {
		return false // Empty nie je automaticky útok
	}

	attackBots := []string{
		"masscan", "nmap", "nuclei", "sqlmap", "nikto",
		"gobuster", "dirb", "burpsuite", "metasploit",
		"exploit", "scanner", "vulnerability", "pentest",
		"hack", "attack", "malware", "botnet",
	}

	userAgentLower := strings.ToLower(userAgent)

	for _, bot := range attackBots {
		if strings.Contains(userAgentLower, bot) {
			return true
		}
	}

	return false
}

// 🛡️ Helper function pre min()
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 🛡️ HELPER FUNCTIONS (unchanged)
func (sm *SecurityMiddleware) incrementSuspiciousCount(ip string) int {
	if sm.client == nil {
		return 1
	}

	key := fmt.Sprintf("suspicious:%s", ip)
	val, err := sm.client.Incr(context.Background(), key).Result()
	if err != nil {
		log.Printf("Redis error in incrementSuspiciousCount: %v", err)
		return 1
	}
	sm.client.Expire(context.Background(), key, 10*time.Minute)
	return int(val)
}

func (sm *SecurityMiddleware) incrementBotCount(ip string) int {
	if sm.client == nil {
		return 1
	}

	key := fmt.Sprintf("bot_count:%s", ip)
	val, err := sm.client.Incr(context.Background(), key).Result()
	if err != nil {
		log.Printf("Redis error in incrementBotCount: %v", err)
		return 1
	}
	sm.client.Expire(context.Background(), key, 30*time.Minute)
	return int(val)
}

// 🟢 PLAYER-FRIENDLY: Veľkorysé rate limiting
func (sm *SecurityMiddleware) checkUnauthenticatedRateLimit(ip string) bool {
	if sm.client == nil {
		return true
	}

	key := fmt.Sprintf("unauth_rate:%s", ip)
	val, err := sm.client.Get(context.Background(), key).Result()

	var count int
	if err == redis.Nil {
		count = 0
	} else if err != nil {
		log.Printf("Redis error in checkUnauthenticatedRateLimit: %v", err)
		return true
	} else {
		count, _ = strconv.Atoi(val)
	}

	// 🟢 GENEROUS: 300 req/min pre hráčov
	if count >= 300 {
		return false
	}

	sm.client.Incr(context.Background(), key)
	sm.client.Expire(context.Background(), key, time.Minute)
	return true
}

func isHealthCheckPath(path string) bool {
	healthPaths := []string{
		"/health", "/api/v1/health", "/api/v1/system/health",
		"/ping", "/status", "/api/v1/status", "/api/v1/test",
		"/info", "/api/v1/info",
	}

	for _, healthPath := range healthPaths {
		if path == healthPath {
			return true
		}
	}
	return false
}

func (sm *SecurityMiddleware) saveToRedisBlacklist(ip, reason string) {
	if sm.client == nil {
		log.Printf("🛡️ [SECURITY] Redis not available - IP %s blacklisted in memory only", ip)
		return
	}

	key := fmt.Sprintf("blacklist:%s", ip)
	data := fmt.Sprintf("%s:%d", reason, time.Now().Unix())
	err := sm.client.Set(context.Background(), key, data, 24*time.Hour).Err()
	if err != nil {
		log.Printf("Redis error in saveToRedisBlacklist: %v", err)
		return
	}

	logKey := "security:blacklist_log"
	logEntry := fmt.Sprintf("%s|%s|%s", ip, reason, time.Now().Format("2006-01-02 15:04:05"))
	sm.client.LPush(context.Background(), logKey, logEntry)
	sm.client.LTrim(context.Background(), logKey, 0, 999)

	log.Printf("🛡️ [SECURITY] IP %s saved to Redis blacklist with reason: %s", ip, reason)
}

// 🛡️ STARTUP FUNCTIONS (unchanged but enhanced logging)
func LoadBlacklistFromRedis(client *redis.Client) {
	if client == nil {
		log.Printf("🛡️ [SECURITY] Redis not available - using in-memory blacklist only")
		log.Printf("🛡️ [SECURITY] Loaded %d pre-configured blacklisted IPs", len(blacklistedIPs))
		log.Printf("🟢 [SECURITY] Whitelisted %d development IPs", len(whitelistedIPs))
		log.Printf("🟢 [SECURITY] Player-friendly mode: legitimate apps auto-allowed")
		return
	}

	keys, err := client.Keys(context.Background(), "blacklist:*").Result()
	if err != nil {
		log.Printf("🛡️ [SECURITY] Redis error loading blacklist: %v", err)
		return
	}

	loadedCount := 0
	for _, key := range keys {
		ip := strings.TrimPrefix(key, "blacklist:")
		if ip != "" && !whitelistedIPs[ip] {
			blacklistedIPs[ip] = true
			loadedCount++
		}
	}

	total := len(blacklistedIPs)
	log.Printf("🛡️ [SECURITY] Loaded %d blacklisted IPs from Redis", loadedCount)
	log.Printf("🛡️ [SECURITY] Total blacklisted IPs: %d", total)
	log.Printf("🟢 [SECURITY] Whitelisted %d development IPs", len(whitelistedIPs))
	log.Printf("🟢 [SECURITY] Player-friendly: Flutter/mobile apps auto-allowed")
}

// 🛡️ ADMIN FUNCTIONS (unchanged)
func GetBlacklist() map[string]bool {
	result := make(map[string]bool)
	for ip, status := range blacklistedIPs {
		result[ip] = status
	}
	return result
}

func GetWhitelist() map[string]bool {
	result := make(map[string]bool)
	for ip, status := range whitelistedIPs {
		result[ip] = status
	}
	return result
}

func AddToBlacklist(ip, reason string) {
	if whitelistedIPs[ip] {
		log.Printf("🟢 [SECURITY] Cannot blacklist whitelisted IP: %s", ip)
		return
	}
	blacklistedIPs[ip] = true
	log.Printf("🛡️ [SECURITY] Manually blacklisted IP: %s (reason: %s)", ip, reason)
}

func AddToWhitelist(ip, reason string) {
	whitelistedIPs[ip] = true
	if blacklistedIPs[ip] {
		delete(blacklistedIPs, ip)
		log.Printf("🟢 [SECURITY] Removed %s from blacklist (now whitelisted)", ip)
	}
	log.Printf("🟢 [SECURITY] Manually whitelisted IP: %s (reason: %s)", ip, reason)
}

func RemoveFromBlacklist(ip string) bool {
	if _, exists := blacklistedIPs[ip]; exists {
		delete(blacklistedIPs, ip)
		log.Printf("🛡️ [SECURITY] Removed IP from blacklist: %s", ip)
		return true
	}
	return false
}

func GetSecurityStats() map[string]interface{} {
	return map[string]interface{}{
		"blacklisted_ips":  len(blacklistedIPs),
		"whitelisted_ips":  len(whitelistedIPs),
		"suspicious_paths": len(suspiciousPaths),
		"connect_blocking": "enabled",
		"rate_limiting":    "300 req/min (player-friendly)",
		"bot_detection":    "strict (attack bots only)",
		"auto_blacklist":   "enabled (higher thresholds)",
		"player_friendly":  "enabled (Flutter/mobile apps auto-allowed)",
		"last_updated":     time.Now().Format("2006-01-02 15:04:05"),
	}
}
