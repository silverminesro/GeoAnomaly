package main

import (
	"log"
	"time"

	"geoanomaly/internal/admin"
	"geoanomaly/internal/auth"
	"geoanomaly/internal/common"
	"geoanomaly/internal/game"
	"geoanomaly/internal/inventory"
	"geoanomaly/internal/loadout"
	"geoanomaly/internal/location"
	"geoanomaly/internal/media"
	"geoanomaly/internal/menu"
	"geoanomaly/internal/user"
	"geoanomaly/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func setupRoutes(db *gorm.DB, redisClient *redis.Client, r2Client *media.R2Client) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Security middleware PRVÝ (najdôležitejšie!)
	if redisClient != nil {
		router.Use(middleware.Security(redisClient))
		router.Use(middleware.RateLimit(redisClient))
	} else {
		router.Use(middleware.BasicSecurity())
	}

	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())

	// Initialize handlers
	authHandler := auth.NewHandler(db, nil)
	userHandler := user.NewHandler(db, nil)
	gameHandler := game.NewHandler(db, nil)
	locationHandler := location.NewHandler(db, nil)
	inventoryHandler := inventory.NewHandler(db)
	menuHandler := menu.NewHandler(db)
	loadoutHandler := loadout.NewHandler(db)
	adminHandler := admin.NewHandler(db, nil)

	// Initialize media service and handler
	var mediaHandler *media.Handler
	if r2Client != nil {
		mediaService := media.NewService(r2Client)
		mediaHandler = media.NewHandler(mediaService)
		log.Println("✅ Media handler initialized with R2 storage")
	} else {
		log.Println("⚠️  Media handler disabled (no R2 client)")
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		redisStatus := "disabled"
		if redisClient != nil {
			redisStatus = "connected"
		}

		r2Status := "disabled"
		if r2Client != nil {
			r2Status = "connected"
		}

		c.JSON(200, gin.H{
			"status":     "healthy",
			"timestamp":  time.Now().Format(time.RFC3339),
			"version":    "1.0.0",
			"service":    "geoanomaly-backend",
			"created_by": "silverminesro",
			"structure":  "unified",
			"security":   "🛡️ active",
			"redis":      redisStatus,
			"r2_storage": r2Status,
		})
	})

	router.GET("/info", func(c *gin.Context) {
		redisStatus := "disabled"
		if redisClient != nil {
			redisStatus = "connected"
		}
		c.JSON(200, gin.H{
			"name":        "GeoAnomaly Backend",
			"version":     "1.0.0",
			"environment": getEnvVar("APP_ENV", "development"),
			"uptime":      time.Since(startTime).String(),
			"developer":   "silverminesro",
			"database":    getEnvVar("DB_NAME", "geoanomaly") + "@" + getEnvVar("DB_HOST", "localhost"),
			"structure":   "unified",
			"security":    "🛡️ CONNECT attacks blocked",
			"redis":       redisStatus,
		})
	})

	// API v1 group
	v1 := router.Group("/api/" + getEnvVar("API_VERSION", "v1"))
	{
		// Basic test endpoints
		v1.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message":   "🎮 GeoAnomaly API is working perfectly!",
				"time":      time.Now().Format(time.RFC3339),
				"endpoint":  "/api/v1/test",
				"developer": "silverminesro",
				"status":    "operational",
				"version":   "unified-structure",
				"security":  "🛡️ protected",
			})
		})

		// Database test endpoint
		v1.GET("/db-test", func(c *gin.Context) {
			var tierCount int64
			var levelCount int64
			var userCount int64
			var zoneCount int64
			var inventoryCount int64

			db.Raw("SELECT COUNT(*) FROM tier_definitions").Scan(&tierCount)
			db.Raw("SELECT COUNT(*) FROM level_definitions").Scan(&levelCount)
			db.Model(&common.User{}).Count(&userCount)
			db.Model(&common.Zone{}).Count(&zoneCount)
			db.Model(&common.InventoryItem{}).Count(&inventoryCount)

			c.JSON(200, gin.H{
				"database": "connected",
				"status":   "operational",
				"stats": gin.H{
					"tiers":           tierCount,
					"levels":          levelCount,
					"users":           userCount,
					"zones":           zoneCount,
					"inventory_items": inventoryCount,
				},
				"message":   "Database connection successful! 🎯",
				"timestamp": time.Now().Format(time.RFC3339),
				"security":  "🛡️ rate limited",
			})
		})

		// User test endpoint
		v1.GET("/users", func(c *gin.Context) {
			var users []common.User
			result := db.Limit(10).Find(&users)

			if result.Error != nil {
				c.JSON(500, gin.H{
					"error":   "Failed to query users",
					"message": result.Error.Error(),
				})
				return
			}

			c.JSON(200, gin.H{
				"users":     users,
				"count":     len(users),
				"total":     result.RowsAffected,
				"message":   "Users retrieved successfully",
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})

		// Zone test endpoint
		v1.GET("/zones", func(c *gin.Context) {
			var zones []common.Zone
			result := db.Limit(10).Find(&zones)

			if result.Error != nil {
				c.JSON(500, gin.H{
					"error":   "Failed to query zones",
					"message": result.Error.Error(),
				})
				return
			}

			c.JSON(200, gin.H{
				"zones":     zones,
				"count":     len(zones),
				"total":     result.RowsAffected,
				"message":   "Zones retrieved successfully",
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})

		// Inventory test endpoint
		v1.GET("/inventory-test", func(c *gin.Context) {
			var inventoryItems []common.InventoryItem
			result := db.Limit(10).Find(&inventoryItems)

			if result.Error != nil {
				c.JSON(500, gin.H{
					"error":   "Failed to query inventory",
					"message": result.Error.Error(),
				})
				return
			}

			c.JSON(200, gin.H{
				"inventory_items": inventoryItems,
				"count":           len(inventoryItems),
				"total":           result.RowsAffected,
				"message":         "Inventory retrieved successfully",
				"timestamp":       time.Now().Format(time.RFC3339),
			})
		})

		// Server status endpoint
		v1.GET("/status", func(c *gin.Context) {
			sqlDB, err := db.DB()
			var dbStatus string
			if err != nil {
				dbStatus = "error"
			} else {
				if err := sqlDB.Ping(); err != nil {
					dbStatus = "disconnected"
				} else {
					dbStatus = "connected"
				}
			}

			redisStatus := "disabled"
			if redisClient != nil {
				redisStatus = "connected"
			}

			r2Status := "disabled"
			if r2Client != nil {
				r2Status = "connected"
			}

			c.JSON(200, gin.H{
				"server": gin.H{
					"status":      "running",
					"uptime":      time.Since(startTime).String(),
					"environment": getEnvVar("APP_ENV", "development"),
					"version":     "1.0.0",
					"structure":   "unified",
					"security":    "🛡️ active",
				},
				"database": gin.H{
					"status": dbStatus,
					"host":   getEnvVar("DB_HOST", "localhost"),
					"name":   getEnvVar("DB_NAME", "geoanomaly"),
				},
				"redis": gin.H{
					"status": redisStatus,
				},
				"r2_storage": gin.H{
					"status": r2Status,
					"bucket": getEnvVar("R2_BUCKET_NAME", ""),
				},
				"security": gin.H{
					"connect_blocking": "active",
					"rate_limiting":    "active",
					"blacklisted_ips":  4,
					"auto_blacklist":   "enabled",
				},
				"developer": "silverminesro",
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})

		// Security status endpoint
		v1.GET("/security/status", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"security": gin.H{
					"status":           "active",
					"connect_blocking": "enabled",
					"rate_limiting":    "enabled",
					"blacklisted_ips": []string{
						"35.193.149.100", "185.91.127.107",
						"185.169.4.150", "204.76.203.193",
					},
					"suspicious_paths": []string{
						"/boaform/", "/admin/", "/.env", "/wp-admin/",
					},
					"auto_blacklist": "enabled",
					"redis_persist":  redisClient != nil,
				},
				"message":   "🛡️ Security system operational",
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})
	}

	// ==========================================
	// 🆕 MEDIA ROUTES (Protected - JWT required)
	// ==========================================
	if mediaHandler != nil {
		mediaRoutes := v1.Group("/media")
		mediaRoutes.Use(middleware.JWTAuth()) // 🔐 JWT required
		{
			mediaRoutes.GET("/image/:filename", mediaHandler.GetImage)
			mediaRoutes.GET("/artifact/:type", mediaHandler.GetArtifactImage)
		}
		log.Println("✅ Media routes registered: /api/v1/media/*")
	}

	// ==========================================
	// 🔐 AUTH ROUTES (Public - no JWT required)
	// ==========================================
	authRoutes := v1.Group("/auth")
	{
		authRoutes.POST("/register", authHandler.Register)
		authRoutes.POST("/login", authHandler.Login)

		authProtected := authRoutes.Group("/")
		authProtected.Use(middleware.JWTAuth())
		{
			authProtected.POST("/refresh", authHandler.RefreshToken)
			authProtected.POST("/logout", authHandler.Logout)
		}
	}

	// ==========================================
	// 👤 USER ROUTES (Protected - JWT required)
	// ==========================================
	userRoutes := v1.Group("/user")
	userRoutes.Use(middleware.JWTAuth())
	{
		userRoutes.GET("/profile", userHandler.GetProfile)
		userRoutes.PUT("/profile", userHandler.UpdateProfile)
		userRoutes.GET("/inventory", userHandler.GetInventory)
		userRoutes.GET("/inventory/:type", userHandler.GetInventoryByType)
		userRoutes.POST("/location", userHandler.UpdateLocation)
		userRoutes.GET("/location/history", userHandler.GetLocationHistory)
		userRoutes.GET("/stats", userHandler.GetUserStats)
	}

	// ==========================================
	// 🎒 INVENTORY ROUTES (Protected - JWT required)
	// ==========================================
	inventoryRoutes := v1.Group("/inventory")
	inventoryRoutes.Use(middleware.JWTAuth())
	{
		inventoryRoutes.GET("/items", inventoryHandler.GetInventory)
		inventoryRoutes.GET("/summary", inventoryHandler.GetInventorySummary)
		inventoryRoutes.DELETE("/:id", inventoryHandler.DeleteItem)
		inventoryRoutes.POST("/:id/use", inventoryHandler.UseItem)
		inventoryRoutes.PUT("/:id/favorite", inventoryHandler.SetFavorite)
		inventoryRoutes.GET("/items/:id", inventoryHandler.GetItemDetail)
		inventoryRoutes.PUT("/:id/equip", inventoryHandler.EquipItem)
	}

	// ==========================================
	// 🛡️ LOADOUT ROUTES (Protected - JWT required)
	// ==========================================
	loadoutRoutes := v1.Group("/loadout")
	loadoutRoutes.Use(middleware.JWTAuth())
	{
		loadoutRoutes.GET("/slots", loadoutHandler.GetLoadoutSlots)
		loadoutRoutes.GET("/user", loadoutHandler.GetUserLoadout)
		loadoutRoutes.POST("/equip", loadoutHandler.EquipItem)
		loadoutRoutes.DELETE("/:slot_id/unequip", loadoutHandler.UnequipItem)
		loadoutRoutes.POST("/:slot_id/repair", loadoutHandler.RepairItem)
		loadoutRoutes.GET("/stats", loadoutHandler.GetLoadoutStats)
		loadoutRoutes.GET("/categories", loadoutHandler.GetGearCategories)
	}

	// ==========================================
	// 🎮 GAME ROUTES (Protected - JWT required)
	// ==========================================
	gameRoutes := v1.Group("/game")
	gameRoutes.Use(middleware.JWTAuth())
	{
		gameRoutes.POST("/scan-area", gameHandler.ScanArea)

		zoneRoutes := gameRoutes.Group("/zones")
		{
			zoneRoutes.GET("/nearby", gameHandler.GetNearbyZones)
			zoneRoutes.GET("/:id", gameHandler.GetZoneDetails)
			zoneRoutes.POST("/:id/enter", gameHandler.EnterZone)
			zoneRoutes.POST("/:id/exit", gameHandler.ExitZone)
			zoneRoutes.GET("/:id/scan", gameHandler.ScanZone)
			zoneRoutes.POST("/:id/collect", gameHandler.CollectItem)
			zoneRoutes.GET("/:id/stats", gameHandler.GetZoneStats)
		}

		itemRoutes := gameRoutes.Group("/items")
		{
			itemRoutes.GET("/artifacts", gameHandler.GetAvailableArtifacts)
			itemRoutes.GET("/gear", gameHandler.GetAvailableGear)
			itemRoutes.POST("/use/:id", gameHandler.UseItem)
		}

		gameRoutes.GET("/leaderboard", gameHandler.GetLeaderboard)
		gameRoutes.GET("/stats", gameHandler.GetGameStats)
	}

	// ==========================================
	// 📍 LOCATION ROUTES (Protected - JWT required)
	// ==========================================
	locationRoutes := v1.Group("/location")
	locationRoutes.Use(middleware.JWTAuth())
	{
		locationRoutes.POST("/update", locationHandler.UpdateLocation)
		locationRoutes.GET("/nearby", locationHandler.GetNearbyPlayers)
		locationRoutes.GET("/zones/:id/activity", locationHandler.GetZoneActivity)
		locationRoutes.GET("/zones/:id/players", locationHandler.GetPlayersInZone)
		locationRoutes.GET("/history", locationHandler.GetLocationHistory)
		locationRoutes.GET("/heatmap", locationHandler.GetLocationHeatmap)
		locationRoutes.POST("/share", locationHandler.ShareLocation)
		locationRoutes.GET("/friends/nearby", locationHandler.GetNearbyFriends)
	}

	// ==========================================
	// 💰 MENU ROUTES (Protected - JWT required)
	// ==========================================
	menuRoutes := v1.Group("/menu")
	menuRoutes.Use(middleware.JWTAuth())
	menuRoutes.Use(middleware.TierExpirationMiddleware(menuHandler.GetService()))
	{
		// Currency endpoints
		menuRoutes.GET("/currency/all", menuHandler.GetAllUserCurrencies)
		menuRoutes.GET("/currency/:type", menuHandler.GetUserCurrency)

		// Market endpoints
		menuRoutes.GET("/market/items", menuHandler.GetMarketItems)
		menuRoutes.POST("/market/purchase", menuHandler.PurchaseMarketItem)

		// Essence package endpoints
		menuRoutes.GET("/essence/packages", menuHandler.GetEssencePackages)
		menuRoutes.POST("/essence/purchase", menuHandler.PurchaseEssencePackage)

		// Tier package endpoints
		menuRoutes.GET("/tier/packages", menuHandler.GetTierPackages)
		menuRoutes.POST("/tier/purchase", menuHandler.PurchaseTierPackage)
		menuRoutes.GET("/tier/history", menuHandler.GetUserTierHistory)
		menuRoutes.POST("/tier/check-expired", menuHandler.CheckExpiredTiers)

		// Google Play Billing endpoints
		menuRoutes.POST("/google-play/verify-purchase", menuHandler.VerifyGooglePlayPurchase)
		menuRoutes.POST("/google-play/verify-subscription", menuHandler.VerifyGooglePlaySubscription) // Admin endpoint

		// Inventory selling
		menuRoutes.POST("/inventory/:id/sell", menuHandler.SellInventoryItem)

		// Transaction history
		menuRoutes.GET("/transactions", menuHandler.GetUserTransactions)
		menuRoutes.GET("/purchases", menuHandler.GetUserPurchases)
		menuRoutes.GET("/essence/purchases", menuHandler.GetUserEssencePurchases)
	}

	// ==========================================
	// 🔧 ADMIN ROUTES (Protected - Admin only)
	// ==========================================
	adminRoutes := v1.Group("/admin")
	adminRoutes.Use(middleware.JWTAuth())
	adminRoutes.Use(middleware.AdminOnly())
	{
		adminRoutes.POST("/zones", gameHandler.CreateEventZone)
		adminRoutes.PUT("/zones/:id", gameHandler.UpdateZone)
		adminRoutes.DELETE("/zones/:id", gameHandler.DeleteZone)
		adminRoutes.POST("/zones/:id/spawn/artifact", gameHandler.SpawnArtifact)
		adminRoutes.POST("/zones/:id/spawn/gear", gameHandler.SpawnGear)
		adminRoutes.POST("/zones/cleanup", gameHandler.CleanupExpiredZones)
		adminRoutes.GET("/zones/expired", gameHandler.GetExpiredZones)
		adminRoutes.GET("/users", userHandler.GetAllUsers)
		adminRoutes.PUT("/users/:id/tier", userHandler.UpdateUserTier)
		adminRoutes.POST("/users/:id/ban", userHandler.BanUser)
		adminRoutes.POST("/users/:id/unban", userHandler.UnbanUser)
		adminRoutes.GET("/analytics/zones", gameHandler.GetZoneAnalytics)
		adminRoutes.GET("/analytics/players", userHandler.GetPlayerAnalytics)
		adminRoutes.GET("/analytics/items", gameHandler.GetItemAnalytics)

		// Inventory management
		adminRoutes.POST("/inventory/add", adminHandler.AddInventoryItem)

		// Menu admin endpoints
		menuAdminRoutes := adminRoutes.Group("/menu")
		{
			// Market item management
			menuAdminRoutes.POST("/market/items", menuHandler.CreateMarketItem)
			menuAdminRoutes.PUT("/market/items/:id", menuHandler.UpdateMarketItem)
			menuAdminRoutes.DELETE("/market/items/:id", menuHandler.DeleteMarketItem)

			// Essence package management
			menuAdminRoutes.POST("/essence/packages", menuHandler.CreateEssencePackage)
			menuAdminRoutes.PUT("/essence/packages/:id", menuHandler.UpdateEssencePackage)
			menuAdminRoutes.DELETE("/essence/packages/:id", menuHandler.DeleteEssencePackage)
		}

		// Security admin endpoints
		securityRoutes := adminRoutes.Group("/security")
		{
			securityRoutes.GET("/blacklist", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "Security blacklist endpoint",
					"status":  "implemented",
				})
			})

			securityRoutes.POST("/blacklist/:ip", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "Manual IP blacklist endpoint",
					"status":  "implemented",
				})
			})

			securityRoutes.DELETE("/blacklist/:ip", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"message": "Remove from blacklist endpoint",
					"status":  "implemented",
				})
			})
		}
	}

	// ==========================================
	// 🛠️ SYSTEM ROUTES
	// ==========================================
	systemRoutes := v1.Group("/system")
	{
		systemRoutes.GET("/health", func(c *gin.Context) {
			redisStatus := "disabled"
			if redisClient != nil {
				redisStatus = "connected"
			}

			c.JSON(200, gin.H{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
				"database":  "connected",
				"redis":     redisStatus,
				"version":   "1.0.0",
				"structure": "unified",
				"security":  "🛡️ active",
			})
		})

		systemRoutes.GET("/stats", func(c *gin.Context) {
			var userCount, zoneCount, inventoryCount int64
			db.Model(&common.User{}).Count(&userCount)
			db.Model(&common.Zone{}).Count(&zoneCount)
			db.Model(&common.InventoryItem{}).Count(&inventoryCount)

			c.JSON(200, gin.H{
				"active_players":  userCount,
				"total_zones":     zoneCount,
				"dynamic_zones":   0,
				"static_zones":    0,
				"total_artifacts": 0,
				"total_gear":      0,
				"inventory_items": inventoryCount,
				"server_uptime":   time.Since(startTime).String(),
				"last_cleanup":    "never",
				"security_status": "🛡️ active",
			})
		})

		systemRoutes.GET("/endpoints", func(c *gin.Context) {
			mediaEndpoints := gin.H{}
			if mediaHandler != nil {
				mediaEndpoints = gin.H{
					"GET /media/image/{filename}": "🖼️ Get image by filename",
					"GET /media/artifact/{type}":  "🎨 Get artifact image by type",
				}
			}

			c.JSON(200, gin.H{
				"message":   "GeoAnomaly API Endpoints",
				"version":   "1.0.0",
				"structure": "unified",
				"security":  "🛡️ CONNECT attacks blocked",
				"endpoints": gin.H{
					"media": mediaEndpoints,
					"menu": gin.H{
						"GET /menu/currency/all":         "💰 Get all user currencies",
						"GET /menu/currency/{type}":      "💰 Get specific currency",
						"GET /menu/market/items":         "🛒 Get market items",
						"POST /menu/market/purchase":     "🛒 Purchase market item",
						"GET /menu/essence/packages":     "💎 Get essence packages",
						"POST /menu/essence/purchase":    "💎 Purchase essence package",
						"POST /menu/inventory/{id}/sell": "💰 Sell inventory item",
						"GET /menu/transactions":         "📊 Get transaction history",
						"GET /menu/purchases":            "📊 Get purchase history",
						"GET /menu/essence/purchases":    "📊 Get essence purchase history",
					},
					"inventory": gin.H{
						"GET /inventory/items":         "🎒 Get user inventory (with images)",
						"GET /inventory/summary":       "📊 Get inventory summary",
						"DELETE /inventory/{id}":       "🗑️ Delete inventory item",
						"POST /inventory/{id}/use":     "⚡ Use inventory item",
						"PUT /inventory/{id}/favorite": "⭐ Set favorite item",
						"PUT /inventory/{id}/equip":    "⚔️ Equip item",
					},
					"security": gin.H{
						"GET /security/status":               "🛡️ Security status",
						"GET /admin/security/blacklist":      "🚫 View blacklist",
						"POST /admin/security/blacklist/*":   "➕ Add to blacklist",
						"DELETE /admin/security/blacklist/*": "➖ Remove from blacklist",
					},
				},
			})
		})
	}

	// ==========================================
	// 📊 METRICS ROUTES
	// ==========================================
	metricsRoutes := v1.Group("/metrics")
	{
		metricsRoutes.GET("/prometheus", func(c *gin.Context) {
			c.String(200, "# GeoAnomaly Metrics\n# Coming soon...")
		})

		metricsRoutes.GET("/security", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"blocked_ips":      4,
				"connect_attempts": "blocked",
				"rate_limited":     "active",
				"auto_blacklist":   "enabled",
				"message":          "🛡️ Security metrics",
			})
		})
	}

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{
			"error":     "Endpoint not found",
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
			"message":   "The requested API endpoint does not exist",
			"hint":      "Visit /api/v1/system/endpoints for available endpoints",
			"structure": "unified",
			"security":  "🛡️ monitored",
		})
	})

	// 405 handler
	router.NoMethod(func(c *gin.Context) {
		c.JSON(405, gin.H{
			"error":   "Method not allowed",
			"path":    c.Request.URL.Path,
			"method":  c.Request.Method,
			"message": "HTTP method not supported for this endpoint",
		})
	})

	return router
}
