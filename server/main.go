package main

import (
	"log"
	"net/http"

	"github.com/HighDoping/EinkPhotoFrame/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func startAPIServer(db *gorm.DB, cfg config.Config) {
	router := gin.Default()

	// Use closures to pass the db connection to handlers
	// Serve static files with authentication
	router.GET("/assets/*filepath", func(c *gin.Context) {
		// Check authentication first
		device, err := authDevice(c, db)
		if err != nil {
			c.JSON(http.StatusUnauthorized, errorResponse("Unauthorized access to assets"))
			return
		}

		// If authenticated, serve the requested file
		path := c.Param("filepath")
		c.File(cfg.CacheDir + path)

		log.Printf("Device %s (%s) accessed asset: %s", device.DeviceID, device.DeviceName, path)
	})

	router.POST("/register", func(c *gin.Context) {
		handleRegisterRequest(c, db)
	})

	router.POST("/dev", func(c *gin.Context) {
		handleDeviceRequest(c, db, cfg)
	})

	router.POST("/admin/device_register", func(c *gin.Context) {
		// Admin endpoint, can be used for management tasks
		handleAdminDeviceRegisterRequest(c, db, cfg)
	})

	log.Println("Starting API server on port 8080...")
	log.Fatal(router.RunTLS(":8080",cfg.CertFile, cfg.KeyFile))
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
