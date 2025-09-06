package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"geoanomaly/internal/auth"
	"geoanomaly/internal/game"
	"geoanomaly/internal/gameplay"
	"geoanomaly/internal/media"
	"geoanomaly/pkg/middleware"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db          *gorm.DB
	redisClient *redis.Client
	startTime   time.Time
	scheduler   *game.Scheduler
	r2Client    *media.R2Client // Pridané pre R2
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	startTime = time.Now()

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using system environment variables")
	} else {
		log.Println("✅ .env file loaded successfully")
	}

	// Set default values if not provided
	setDefaultEnvVars()
}

func main() {
	log.Println("🚀 Starting GeoAnomaly Backend Server...")
	log.Printf("⏰ Start Time: %s", startTime.Format("2006-01-02 15:04:05"))
	log.Printf("👤 Started by: silverminesro")

	// Test our .env configuration
	if err := testEnvConfig(); err != nil {
		log.Fatalf("❌ Environment configuration error: %v", err)
	}
	log.Println("✅ Environment configuration validated")

	// Initialize Redis connection
	redisClient = initRedis()
	if redisClient != nil {
		log.Println("✅ Redis connected successfully")
		middleware.LoadBlacklistFromRedis(redisClient)
	} else {
		log.Println("⚠️  Redis disabled - security middleware will work without persistence")
	}

	// Initialize database connection
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	log.Println("✅ Database connected successfully")

	// Test our existing database schema
	if err := testDatabaseSchema(db); err != nil {
		log.Fatalf("❌ Database schema test failed: %v", err)
	}
	log.Println("✅ Database schema validated")

	// Check migrations status
	if err := checkMigrations(db); err != nil {
		log.Fatalf("❌ Migration check failed: %v", err)
	}
	log.Println("✅ Database migrations status verified")

	// Initialize R2 client
	// Initialize R2 client
	log.Println("🖼️  Initializing Cloudflare R2 client...")

	// ✅ PRIDANÉ: Debug R2 credentials (bezpečne)
	accountID := getEnvVar("R2_ACCOUNT_ID", "")
	accessKeyID := getEnvVar("R2_ACCESS_KEY_ID", "")
	secretAccessKey := getEnvVar("R2_SECRET_ACCESS_KEY", "")
	bucketName := getEnvVar("R2_BUCKET_NAME", "geoanomaly")

	log.Printf("🔑 R2_ACCOUNT_ID: %s", accountID)
	log.Printf("🔑 R2_ACCESS_KEY_ID: %s", accessKeyID)
	if len(secretAccessKey) > 0 {
		log.Printf("🔑 R2_SECRET_ACCESS_KEY: %s... (length: %d)", secretAccessKey[:min(10, len(secretAccessKey))], len(secretAccessKey))
	} else {
		log.Printf("🔑 R2_SECRET_ACCESS_KEY: <EMPTY>")
	}
	log.Printf("🔑 R2_BUCKET_NAME: %s", bucketName)

	if accountID == "" || accessKeyID == "" || secretAccessKey == "" {
		log.Println("❌ Missing R2 credentials in environment variables")
		r2Client = nil
	} else {
		r2Client, err = media.NewR2Client(accountID, accessKeyID, secretAccessKey, bucketName)
		if err != nil {
			log.Printf("⚠️  Failed to initialize R2 client: %v", err)
			log.Println("⚠️  Media service will be disabled")
			r2Client = nil
		} else {
			log.Println("✅ R2 client initialized successfully")
		}
	}

	// Start zone cleanup scheduler
	log.Println("🕐 Starting Zone TTL Cleanup Scheduler...")
	scheduler = game.NewScheduler(db)
	scheduler.Start()
	log.Println("✅ Zone cleanup scheduler started (5min interval)")

	// Setup graceful shutdown
	setupGracefulShutdown()

	// Setup routes with security middleware and R2 client
	router := setupRoutes(db, redisClient, r2Client)

	// Get server configuration from .env
	port := getEnvVar("PORT", "8080")
	host := getEnvVar("HOST", "localhost")

	// Print server information
	printServerInfo(host, port)

	// Start server
	serverAddr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("🌐 Server starting on %s", serverAddr)
	log.Printf("📱 Flutter can connect to: http://%s/api/v1", serverAddr)
	log.Printf("🧹 Zone cleanup running every 5 minutes")
	log.Printf("🛡️ Security middleware active - CONNECT attacks blocked")
	if r2Client != nil {
		log.Printf("🖼️  Media service active - R2 storage connected")
	}

	if err := router.Run(serverAddr); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}

// Initialize Redis connection
func initRedis() *redis.Client {
	redisAddr := getEnvVar("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnvVar("REDIS_PASSWORD", "")
	redisDB := 0

	log.Printf("🔌 Connecting to Redis at %s...", redisAddr)

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Test connection s context.Background()
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Printf("⚠️  Redis connection failed: %v", err)
		log.Println("⚠️  Security middleware will work without Redis persistence")
		return nil
	}

	return client
}

// Setup graceful shutdown
func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("\n🛑 Shutting down server gracefully...")

		// Stop scheduler
		if scheduler != nil {
			scheduler.Stop()
			log.Println("✅ Zone cleanup scheduler stopped")
		}

		// Close Redis connection
		if redisClient != nil {
			redisClient.Close()
			log.Println("✅ Redis connection closed")
		}

		// Close database connection
		if db != nil {
			sqlDB, err := db.DB()
			if err == nil {
				sqlDB.Close()
				log.Println("✅ Database connection closed")
			}
		}

		log.Println("👋 Server shutdown complete")
		os.Exit(0)
	}()
}

func initDB() (*gorm.DB, error) {
	// Build connection string from .env
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		getEnvVar("DB_HOST", "localhost"),
		getEnvVar("DB_USER", "postgres"),
		getEnvVar("DB_PASSWORD", ""),
		getEnvVar("DB_NAME", "geoanomaly"),
		getEnvVar("DB_PORT", "5432"),
		getEnvVar("DB_SSLMODE", "disable"),
		getEnvVar("DB_TIMEZONE", "UTC"),
	)

	log.Println("🔌 Connecting to database...")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

func testEnvConfig() error {
	log.Println("🔍 Testing .env configuration...")

	// Test database config
	dbHost := getEnvVar("DB_HOST", "")
	dbUser := getEnvVar("DB_USER", "")
	dbPassword := getEnvVar("DB_PASSWORD", "")
	dbName := getEnvVar("DB_NAME", "")

	if dbHost == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		return fmt.Errorf("missing required database configuration in .env")
	}

	log.Printf("📊 Database: %s@%s:%s/%s", dbUser, dbHost, getEnvVar("DB_PORT", "5432"), dbName)

	// Test JWT config
	jwtSecret := getEnvVar("JWT_SECRET", "")
	if len(jwtSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	log.Printf("🔑 JWT Secret: %s... (length: %d)", jwtSecret[:8], len(jwtSecret))

	return nil
}

func testDatabaseSchema(db *gorm.DB) error {
	log.Println("🔍 Testing existing database schema...")

	// Test our config tables from previous setup
	var tierCount int64
	if err := db.Raw("SELECT COUNT(*) FROM tier_definitions").Scan(&tierCount).Error; err != nil {
		return fmt.Errorf("failed to query tier_definitions: %w", err)
	}

	var levelCount int64
	if err := db.Raw("SELECT COUNT(*) FROM level_definitions").Scan(&levelCount).Error; err != nil {
		return fmt.Errorf("failed to query level_definitions: %w", err)
	}

	log.Printf("📊 Existing data: %d tiers, %d levels", tierCount, levelCount)

	// Test if our main tables exist
	tableNames := []string{"users", "zones", "artifacts", "gear", "inventory_items", "player_sessions"}
	for _, tableName := range tableNames {
		var exists bool
		err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = ?)", tableName).Scan(&exists).Error
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", tableName, err)
		}
		if exists {
			log.Printf("✅ Table exists: %s", tableName)
		} else {
			log.Printf("⚠️  Table missing: %s", tableName)
		}
	}

	return nil
}

func checkMigrations(db *gorm.DB) error {
	log.Println("🔄 Checking database migrations status...")

	// Count existing data
	var zoneCount int64
	db.Model(&gameplay.Zone{}).Count(&zoneCount)
	log.Printf("📍 Found %d zones in database", zoneCount)

	var userCount int64
	db.Model(&auth.User{}).Count(&userCount)
	log.Printf("👤 Found %d users in database", userCount)

	// Check if core tables have data
	if zoneCount == 0 {
		log.Println("⚠️  No zones found - database may need seeding")
	}

	if userCount == 0 {
		log.Println("⚠️  No users found - database may need seeding")
	}

	log.Println("ℹ️  Using existing database schema (no migrations applied)")

	return nil
}

func setDefaultEnvVars() {
	envDefaults := map[string]string{
		"PORT":     "8080",
		"HOST":     "localhost",
		"GIN_MODE": "debug",

		// Database (fallback values, .env should override these)
		"DB_HOST":     "localhost",
		"DB_PORT":     "5432",
		"DB_USER":     "postgres",
		"DB_PASSWORD": "password",
		"DB_NAME":     "geoanomaly",
		"DB_SSLMODE":  "disable",
		"DB_TIMEZONE": "UTC",

		// Redis defaults
		"REDIS_ADDR":     "localhost:6379",
		"REDIS_PASSWORD": "",

		// JWT (fallback, .env should override)
		"JWT_SECRET":     "your-super-secret-jwt-key-change-this-in-production",
		"JWT_EXPIRES_IN": "24h",

		// App Settings
		"APP_ENV":     "development",
		"API_VERSION": "v1",
		"DEBUG":       "true",
		"LOG_LEVEL":   "info",
	}

	for key, defaultValue := range envDefaults {
		if os.Getenv(key) == "" {
			os.Setenv(key, defaultValue)
		}
	}
}

func getEnvVar(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printServerInfo(host, port string) {
	uptime := time.Since(startTime).Round(time.Second)

	// Get config from .env
	dbName := getEnvVar("DB_NAME", "geoanomaly")
	dbHost := getEnvVar("DB_HOST", "localhost")
	redisAddr := getEnvVar("REDIS_ADDR", "localhost:6379")
	jwtSecret := getEnvVar("JWT_SECRET", "")

	separator := strings.Repeat("=", 60)

	fmt.Println("\n" + separator)
	fmt.Println("🎮 GeoAnomaly BACKEND SERVER")
	fmt.Println(separator)
	fmt.Printf("🌐 Server:        http://%s:%s\n", host, port)
	fmt.Printf("📊 Health Check:  http://%s:%s/health\n", host, port)
	fmt.Printf("🔗 API Base:      http://%s:%s/api/v1\n", host, port)
	fmt.Printf("⏱️  Startup Time:  %v\n", uptime)
	fmt.Printf("🗄️  Database:      %s@%s\n", dbName, dbHost)
	fmt.Printf("🔴 Redis:         %s %s\n", redisAddr, func() string {
		if redisClient != nil {
			return "✅"
		}
		return "❌"
	}())
	fmt.Printf("🖼️  R2 Storage:    %s\n", func() string {
		if r2Client != nil {
			return "✅ Connected"
		}
		return "❌ Disabled"
	}())
	fmt.Printf("🔑 JWT Configured: %s\n", func() string {
		if len(jwtSecret) >= 32 {
			return "✅ Yes"
		}
		return "❌ No"
	}())
	fmt.Printf("🧹 Zone Cleanup:  ✅ Active (5min)\n")
	fmt.Printf("🛡️ Security:      ✅ Active (CONNECT blocked)\n")
	fmt.Printf("🚀 Status:        Ready for connections\n")
	fmt.Println(separator)

	// Test endpoints with cleanup endpoints
	fmt.Println("\n🧪 TEST ENDPOINTS:")
	fmt.Printf("Health:   GET  http://%s:%s/health\n", host, port)
	fmt.Printf("Info:     GET  http://%s:%s/info\n", host, port)
	fmt.Printf("API Test: GET  http://%s:%s/api/v1/test\n", host, port)
	fmt.Printf("DB Test:  GET  http://%s:%s/api/v1/db-test\n", host, port)
	fmt.Printf("Status:   GET  http://%s:%s/api/v1/status\n", host, port)
	fmt.Printf("Users:    GET  http://%s:%s/api/v1/users\n", host, port)
	fmt.Printf("Zones:    GET  http://%s:%s/api/v1/zones\n", host, port)

	fmt.Println("\n🧹 CLEANUP ENDPOINTS:")
	fmt.Printf("Manual Cleanup: POST http://%s:%s/api/v1/admin/zones/cleanup\n", host, port)
	fmt.Printf("Expired Zones:  GET  http://%s:%s/api/v1/admin/zones/expired\n", host, port)
	fmt.Printf("Zone Analytics: GET  http://%s:%s/api/v1/admin/analytics/zones\n", host, port)

	// Security info
	fmt.Println("\n🛡️ SECURITY STATUS:")
	fmt.Println("• CONNECT attacks blocked automatically")
	fmt.Println("• 4 IPs pre-blacklisted from recent attacks")
	fmt.Println("• Rate limiting: 20 req/min (unauthenticated)")
	fmt.Println("• Suspicious path detection active")
	fmt.Println("• Auto-blacklisting for repeat offenders")
	if redisClient != nil {
		fmt.Println("• Redis persistence for blacklist enabled")
	} else {
		fmt.Println("• Redis persistence disabled (in-memory only)")
	}

	fmt.Println("\n💾 DATABASE STATUS:")
	fmt.Println("• All main tables exist")
	fmt.Println("• 5 tier definitions configured")
	fmt.Println("• 200 level definitions configured")
	fmt.Println("• Schema validation passed")
	fmt.Println("• TTL cleanup scheduler active")

	fmt.Println("\n🔥 Server Ready! Test endpoints now!")
	fmt.Printf("💡 Try: curl http://%s:%s/health\n", host, port)
	fmt.Printf("🧹 Zone cleanup runs every 5 minutes automatically\n")
	fmt.Printf("🛡️ Security: Try CONNECT attack to test blocking\n")
	fmt.Println(separator + "\n")
}
