package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var jwtMasterKey []byte
var adminKey string
var imageDir string
var imageDirRefresh int
var cacheDir string

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}
	// Get admin key from environment
	adminKey = os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		log.Println("Warning: ADMIN_TOKEN not set in .env, using default (not secure for production)")
		adminKey = "default_admin_token"
	}
	log.Println("Using admin token:", adminKey)
	imageDir = os.Getenv("IMAGE_DIR")
	if imageDir == "" {
		log.Println("Warning: IMAGE_DIR not set in .env, using default")
		imageDir = "./images"
	}
	log.Println("Using image directory:", imageDir)
	imageDirRefresh, err = strconv.Atoi(os.Getenv("IMAGE_DIR_REFRESH"))
	if err != nil {
		log.Println("Warning: IMAGE_DIR_REFRESH not set in .env, using default")
		imageDirRefresh = 86400
	}
	cacheDir = os.Getenv("CACHE_DIR")
	if cacheDir == "" {
		log.Println("Warning: CACHE_DIR not set in .env, using default")
		cacheDir, _ = os.UserCacheDir()
	}
}

func startAPIServer(db *gorm.DB) {
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
		c.File(cacheDir + path)

		log.Printf("Device %s (%s) accessed asset: %s", device.DeviceID, device.DeviceName, path)
	})

	router.POST("/register", func(c *gin.Context) {
		handleRegisterRequest(c, db)
	})

	router.POST("/dev", func(c *gin.Context) {
		handleDeviceRequest(c, db)
	})

	router.POST("/admin/device_register", func(c *gin.Context) {
		// Admin endpoint, can be used for management tasks
		handleAdminDeviceRegisterRequest(c, db)
	})

	log.Println("Starting API server on port 8080...")
	log.Fatal(router.RunTLS(":8080", "cert.pem", "key.pem"))
}

func main() {
	db, err := dbInit()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbClose(db)

	if err := refreshImages(db); err != nil {
		log.Fatalf("Failed to refresh images: %v", err)
	}

	if err := updateRandomList(db); err != nil {
		log.Fatalf("Failed to update random image list: %v", err)
	}

	// Start API server
	startAPIServer(db)
}
