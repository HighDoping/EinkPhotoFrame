package main

import (
	_ "embed"
	"log"
	"net/http"

	"github.com/HighDoping/EinkPhotoFrame/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

//go:embed admin.html
var adminPage []byte

func startAPIServer(db *gorm.DB, cfg config.Config) {
	router := gin.Default()

	// Generated assets are private and may only be read by their assignee.
	router.GET("/assets/:assetUUID", func(c *gin.Context) {
		device, err := authDevice(c, db)
		if err != nil {
			c.JSON(http.StatusUnauthorized, errorResponse("Unauthorized access to assets"))
			return
		}

		var assignment DeviceImage
		if err := db.Where("asset_uuid = ? AND device_id = ?", c.Param("assetUUID"), device.DeviceID).First(&assignment).Error; err != nil {
			c.JSON(http.StatusNotFound, errorResponse("Asset not available for this device"))
			return
		}
		c.File(assignment.Path)

		log.Printf("Device %s (%s) accessed assigned asset: %s", device.DeviceID, device.DeviceName, assignment.AssetUUID)
	})

	router.POST("/register", func(c *gin.Context) {
		handleRegisterRequest(c, db)
	})

	router.POST("/dev", func(c *gin.Context) {
		handleDeviceRequest(c, db, cfg)
	})

	router.POST("/admin/device_register", func(c *gin.Context) {
		// Backwards-compatible JSON endpoint for provisioning devices.
		handleAdminDeviceRegisterRequest(c, db, cfg)
	})

	router.GET("/admin", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", adminPage) })
	admin := router.Group("/admin/api", requireAdmin(cfg))
	admin.GET("/devices", func(c *gin.Context) { handleAdminDevices(c, db) })
	admin.GET("/devices/:deviceID/current-image", func(c *gin.Context) { handleAdminCurrentImage(c, db) })
	admin.POST("/devices", func(c *gin.Context) { handleAdminCreateDevice(c, db) })
	admin.PATCH("/devices/:deviceID", func(c *gin.Context) { handleAdminUpdateDevice(c, db) })
	admin.POST("/devices/:deviceID/revoke", func(c *gin.Context) { handleAdminRevokeDevice(c, db) })
	admin.POST("/devices/:deviceID/enable", func(c *gin.Context) { handleAdminEnableDevice(c, db) })
	admin.DELETE("/devices/:deviceID", func(c *gin.Context) { handleAdminDeleteDevice(c, db) })
	admin.POST("/devices/:deviceID/reset-image", func(c *gin.Context) { handleAdminResetDeviceImage(c, db) })

	log.Println("Starting API server on port 8080...")
	log.Fatal(router.RunTLS(":8080", cfg.CertFile, cfg.KeyFile))
}

func main() {
	cfg := config.LoadFromEnv()

	db, err := dbInit()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbClose(db)

	if err := refreshImages(db, cfg); err != nil {
		log.Fatalf("Failed to refresh images: %v", err)
	}

	if err := updateRandomList(db); err != nil {
		log.Fatalf("Failed to update random image list: %v", err)
	}

	// Start API server
	startAPIServer(db, cfg)
}
