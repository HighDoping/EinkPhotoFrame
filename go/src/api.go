package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var jwtMasterKey []byte
var adminKey string

// Define models
type Device struct {
    ID         uint `gorm:"primarykey"`
    DeviceID   string `gorm:"uniqueIndex;not null"`
    DeviceName string `gorm:"not null"`
    DeviceToken string `gorm:"uniqueIndex;not null"`
    CurrentImage string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type DeviceSetting struct {
    ID              uint `gorm:"primarykey"`
    DeviceID        string `gorm:"not null"`
    ImgUpdateInterval int `gorm:"not null;default:600"`
    Height          int `gorm:"not null;default:480"`
    Width           int `gorm:"not null;default:800"`
    Rotation        int `gorm:"not null;default:0"`
    Palette         string `gorm:"not null;default:'7Standard'"`
    DitherAlgorithm string `gorm:"not null;default:'StevenPigeon'"`
    DitherStrength  float64 `gorm:"not null;default:1.0"`
    CreatedAt       time.Time
    UpdatedAt       time.Time
    Device          Device `gorm:"foreignKey:DeviceID;references:DeviceID"`
}

type DeviceTelemetry struct {
    ID          uint `gorm:"primarykey"`
    DeviceID    string `gorm:"not null"`
    BatteryLevel int `gorm:"not null;default:100"`
    LastSeen    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
    Device      Device `gorm:"foreignKey:DeviceID;references:DeviceID"`
}

type Image struct {
    ID        uint `gorm:"primarykey"`
    Path      string `gorm:"uniqueIndex;not null"`
    UUID      string `gorm:"uniqueIndex;not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

type DitheredImage struct {
    ID              uint `gorm:"primarykey"`
    UUID            string `gorm:"not null"`
    Palette         string `gorm:"not null"`
    DitherAlgorithm string `gorm:"not null"`
    DitherStrength  float64 `gorm:"not null;default:1.0"`
    Path            string `gorm:"uniqueIndex;not null"`
    CreatedAt       time.Time
    UpdatedAt       time.Time
    Image           Image `gorm:"foreignKey:UUID;references:UUID"`
}

type RandomImage struct {
    ID   uint `gorm:"primarykey"`
    UUID string `gorm:"uniqueIndex;not null"`
}

type jwtCustomClaims struct {
    DeviceID    string `json:"device_id"`
    DeviceToken string `json:"device_token"`
    DeviceName  string `json:"device_name"`
    jwt.RegisteredClaims
}

    
func init() {
    // Load .env file
    err := godotenv.Load()
    if err != nil {
        log.Println("Warning: Error loading .env file:", err)
    }

    // Get JWT master key from environment
    jwtKey := os.Getenv("JWT_MASTER_KEY")
    if jwtKey == "" {
        log.Println("Warning: JWT_MASTER_KEY not set in .env, using default (not secure for production)")
        jwtKey = "default_insecure_key"
    }
    jwtMasterKey = []byte(jwtKey)

    log.Println("Using JWT master key:", jwtKey)
    // Get admin key from environment
    adminKey= os.Getenv("ADMIN_KEY")
    if adminKey == "" {
        log.Println("Warning: ADMIN_TOKEN not set in .env, using default (not secure for production)")
        adminKey = "default_admin_token"
    }
    log.Println("Using admin token:", adminKey)
}



func handleRegisterRequest(c *gin.Context,db *gorm.DB) error {
    // Register a new device if authorized in database, returns a JWT token
    var requestData map[string]interface{}
    if err := c.BindJSON(&requestData); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
        return err
    }
    deviceID, ok := requestData["device_id"].(string)
    if !ok || deviceID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
        return fmt.Errorf("device_id is required")
    }
    deviceToken, ok := requestData["device_token"].(string)
    if !ok || deviceToken == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "device_token is required"})
        return fmt.Errorf("device_token is required")
    }
    deviceName, ok := requestData["device_name"].(string)
    if !ok || deviceName == "" {
        deviceName = deviceID // Use device_id as default name if not provided
    }
    // Check if device already exists
    var existingDevice Device
    result := db.Where("device_id = ?, device_token = ?", deviceID, deviceToken).First(&existingDevice)
    if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
        log.Printf("Error checking existing device: %v", result.Error)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return result.Error
    }
    if result.RowsAffected > 0 {
        // Device already exists, return JWT token
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtCustomClaims{
            DeviceID:    existingDevice.DeviceID,
            DeviceToken: existingDevice.DeviceToken,
            DeviceName:  existingDevice.DeviceName,
            RegisteredClaims: jwt.RegisteredClaims{
                Issuer:    "device_api",
                IssuedAt: jwt.NewNumericDate(time.Now()),
                ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Token valid for 24 hours
            },
        })
        tokenString, err := token.SignedString(jwtMasterKey)
        if err != nil {
            log.Printf("Error signing token: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return err
        }
        c.JSON(http.StatusOK, gin.H{"message": "Device registered", "token": tokenString})
        log.Printf("Device registered: %s (%s)", existingDevice.DeviceID, existingDevice.DeviceName)
        return nil
    }
    // Device does not exist, deny registration
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized device registration"})
    log.Printf("Unauthorized device registration attempt: %s", deviceID)
    return fmt.Errorf("unauthorized device registration")
}

func handleAdminDeviceRegisterRequest(c *gin.Context,db *gorm.DB) {
    // Check if the request has a valid admin key
    if !checkAdminKey(c) {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized access"})
        return // Unauthorized access
    }
    // Admin logic here, for now just return a success message
    var requestData map[string]interface{}
    if err := c.BindJSON(&requestData); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
        return
    }
    // Process the request data as needed
    device_id:= requestData["device_id"].(string)
    device_token := requestData["device_token"].(string)
    if device_id == "" || device_token == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "device_id and device_token are required"})
        return
    }
    device_name := device_id
    if name, ok := requestData["device_name"].(string); ok && name != "" {
        device_name = name
    }
    // Insert the device into the database
    device := Device{
        DeviceID:   device_id,
        DeviceName: device_name,
        DeviceToken: device_token,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    result := db.Create(&device)
    if result.Error != nil {
        log.Printf("Error inserting device: %v", result.Error)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Device registered successfully", "device_id": device.DeviceID, "device_token": device.DeviceToken, "device_name": device.DeviceName})
    log.Printf("Device registered: %s (%s)", device.DeviceID, device.DeviceName)
}

func checkAdminKey(c *gin.Context) bool {
    // Check if the request has a valid admin key
    tokenString := c.GetHeader("Authorization")
    if tokenString == "" {
        return false
    }
    tokenString = tokenString[len("Bearer "):] // Remove "Bearer " prefix
    if tokenString != adminKey {
        return false
    }
    return true
}

func updateLastSeen(device Device , db *gorm.DB) error {
    // Update the last seen timestamp for the device
    deviceTelemetry := DeviceTelemetry{
        DeviceID: device.DeviceID,
        LastSeen: time.Now(),
    }
    result := db.Save(&deviceTelemetry)
    if result.Error != nil {
        log.Printf("Error updating last seen for device %s: %v", device.DeviceID, result.Error)
        return result.Error
    }
    log.Printf("Last seen updated for device %s at %v", device.DeviceID, deviceTelemetry.LastSeen)
    return nil
}


func getBearerToken(c *gin.Context) (string, error) {
    // Extract the Bearer token from the Authorization header
    tokenString := c.GetHeader("Authorization")
    if tokenString == "" {
        return "", fmt.Errorf("authorization header missing")
    }
    if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
        return "", fmt.Errorf("invalid authorization header format")
    }
    return tokenString[7:], nil
}

func getJWTClaims(c *gin.Context) (jwt.Claims, error) {
    // Extract and parse the JWT token from the Authorization header
    tokenString, err := getBearerToken(c)
    if err != nil {
        return nil, err
    }
    token, err := jwt.ParseWithClaims(tokenString, &jwtCustomClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return jwtMasterKey, nil
    })
    if err != nil || !token.Valid {
        return nil, fmt.Errorf("invalid token: %v", err)
    }
    claims, ok := token.Claims.(*jwtCustomClaims)
    if !ok {
        return nil, fmt.Errorf("invalid token claims")
    }
    return claims, nil
    //Can add expiration check here if needed
}

func authDevice(c *gin.Context, db *gorm.DB) (Device, jwt.Claims, error) {
    // Check if the request has a valid device token via JWT, returns device details
    claims, err := getJWTClaims(c)
    if err != nil {
        log.Printf("Error getting JWT claims: %v", err)
        return Device{}, nil, err
    }
    deviceID:= claims.(*jwtCustomClaims).DeviceID
    deviceToken := claims.(*jwtCustomClaims).DeviceToken

    // Fetch device details from the database
    var device Device
    result := db.Where("device_id = ? AND device_token = ?", deviceID, deviceToken).First(&device)
    if result.Error != nil {
        log.Printf("Error fetching device: %v", result.Error)
        return Device{}, nil, result.Error
    }
    if device.DeviceID == "" {
        return Device{}, nil, fmt.Errorf("device not found")
    }
    log.Printf("Device authenticated: %s (%s)", device.DeviceID, device.DeviceName)
    // Update last seen timestamp for the device
    err = updateLastSeen(device, db)
    if err != nil {
        log.Printf("Error updating last seen for device %s: %v", device.DeviceID, err)
        return Device{}, nil, err
    }
    // Return device details and claims
    return device, claims, nil
}


func refreshJWT(c *gin.Context, db *gorm.DB) (string,error) {
    // Refresh the JWT token for the device
    device, claims, err := authDevice(c, db)
    if err != nil {
        return "", err
    }
    // Check expiration of the current token
    if claims == nil {
        return "", fmt.Errorf("invalid token claims")
    }
    if claims.(*jwtCustomClaims).ExpiresAt.Time.Before(time.Now().Add(-time.Hour)) {
        // Create a new JWT token with the same claims
        newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtCustomClaims{
            DeviceID:    device.DeviceID,
            DeviceToken: device.DeviceToken,
            DeviceName:  device.DeviceName,
            RegisteredClaims: jwt.RegisteredClaims{
                Issuer:    "device_api",
                IssuedAt: jwt.NewNumericDate(time.Now()),
                ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Token valid for 24 hours
            },
        })
        newTokenString, err := newToken.SignedString(jwtMasterKey)
        if err != nil {
            log.Printf("Error signing new token: %v", err)
            return "", err
        }
        return newTokenString, nil}
    // If the token is still valid, return the existing token
    return "", fmt.Errorf("token is still valid, no refresh needed")
}

func handleDeviceRequest(c *gin.Context, db *gorm.DB) {
    device, _, err := authDevice(c, db)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized device"})
        return
    }
    //get json body
    var requestData map[string]interface{}
    if err := c.BindJSON(&requestData); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
        return
    }
    if requestData["action"] == "refresh_token" {
        // Refresh the JWT token
        newToken, err := refreshJWT(c, db)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
            return
        }
        c.JSON(http.StatusOK, gin.H{"message": "Token refreshed", "token": newToken})
        return
    }
    if requestData["action"] == "get_settings" {
        // Get device settings
        var settings DeviceSetting
        result := db.Where("device_id = ?", device.DeviceID).First(&settings)
        if result.Error != nil {
            if result.Error == gorm.ErrRecordNotFound {
                c.JSON(http.StatusNotFound, gin.H{"error": "Settings not found"})
            } else {
                log.Printf("Error fetching settings: %v", result.Error)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            }
            return
        }
        c.JSON(http.StatusOK, gin.H{"settings": settings})
        return
    }
    if requestData["action"] == "update_settings" {
        // Update device settings
        var settings DeviceSetting
        result := db.Where("device_id = ?", device.DeviceID).First(&settings)
        if result.Error != nil {
            if result.Error == gorm.ErrRecordNotFound {
                // Create new settings if not found
                settings = DeviceSetting{
                    DeviceID: device.DeviceID,
                    CreatedAt: time.Now(),
                }
            } else {
                log.Printf("Error fetching settings: %v", result.Error)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
                return
            }
        }
        // Update settings with request data
        if imgUpdateInterval, ok := requestData["img_update_interval"].(int); ok {
            settings.ImgUpdateInterval = imgUpdateInterval
        }
        if height, ok := requestData["height"].(int); ok {
            settings.Height = height
        }
        if width, ok := requestData["width"].(int); ok {
            settings.Width = width
        }
        if rotation, ok := requestData["rotation"].(int); ok {
            settings.Rotation = rotation
        }
        if palette, ok := requestData["palette"].(string); ok {
            settings.Palette = palette
        }
        if ditherAlgorithm, ok := requestData["dither_algorithm"].(string); ok {
            settings.DitherAlgorithm = ditherAlgorithm
        }
        if ditherStrength, ok := requestData["dither_strength"].(float64); ok {
            settings.DitherStrength = ditherStrength
        }
        
        // Save updated settings to database
        settings.UpdatedAt = time.Now()
        result = db.Save(&settings)
        if result.Error != nil {
            log.Printf("Error saving settings: %v", result.Error)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully", "settings": settings})
        return
    }
    if requestData["action"] == "update_telemetry" {
        // Update device telemetry
        var telemetry DeviceTelemetry
        result := db.Where("device_id = ?", device.DeviceID).First(&telemetry)
        if result.Error != nil {
            if result.Error == gorm.ErrRecordNotFound {
                // Create new telemetry if not found
                telemetry = DeviceTelemetry{
                    DeviceID: device.DeviceID,
                    LastSeen: time.Now(),
                }
            } else {
                log.Printf("Error fetching telemetry: %v", result.Error)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
                return
            }
        }
        // Update telemetry with request data
        if batteryLevel, ok := requestData["battery_level"].(int); ok {
            telemetry.BatteryLevel = batteryLevel
        }
        telemetry.LastSeen = time.Now()
        
        // Save updated telemetry to database
        result = db.Save(&telemetry)
        if result.Error != nil {
            log.Printf("Error saving telemetry: %v", result.Error)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        
        c.JSON(http.StatusOK, gin.H{"message": "Telemetry updated successfully", "telemetry": telemetry})
        return
    }
    if requestData["action"] == "get_image" {
        // Get current image for the device
        // get current image, updated at, and image update interval
        var settings DeviceSetting
        result := db.Where("device_id = ?", device.DeviceID).First(&settings)
        if result.Error != nil {
            if result.Error == gorm.ErrRecordNotFound {
                c.JSON(http.StatusNotFound, gin.H{"error": "Settings not found"})
            } else {
                log.Printf("Error fetching settings: %v", result.Error)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            }
            return
        }
        if device.UpdatedAt.Add(time.Duration(settings.ImgUpdateInterval)*time.Second).After(time.Now()) && device.CurrentImage != "" {
            // If the last update time plus interval is less than current time, return no update message
            c.JSON(http.StatusOK, gin.H{"message": "No image update needed"})
            return
        } else{
            // Return updated image
            return
        }
    }
    if requestData["action"] == "update_image" {
        // Update current image for the device
        //skip
        
    }
}


func dbInit() (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open("./db.db"), &gorm.Config{})
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    


    // Auto migrate schemas
    err = db.AutoMigrate(&Device{}, &DeviceSetting{}, &DeviceTelemetry{}, &Image{}, &DitheredImage{}, &RandomImage{})
    if err != nil {
        return nil, fmt.Errorf("failed to migrate database schema: %w", err)
    }
    
    return db, nil
}

func refreshImages(db *gorm.DB) error {

    
    imagePaths, err := generateFileList("../asset", []string{".jpg", ".jpeg", ".png", ".bmp"})
    if err != nil {
        return fmt.Errorf("failed to generate file list: %w", err)
    }

    for _, path := range imagePaths {
        // Check if image already exists
        var count int64
        db.Model(&Image{}).Where("path = ?", path).Count(&count)
        if count > 0 {
            continue // Skip if exists
        }
        
        // Create new image
        uuid := generateUUID()
        image := Image{
            Path: path,
            UUID: uuid,
        }
        
        result := db.Create(&image)
        if result.Error != nil {
            log.Printf("failed to insert image %s into database: %v\n", path, result.Error)
            continue
        }
        fmt.Printf("Inserted image: %s with UUID: %s\n", path, uuid)
    }
    return nil
}

func dbClose(db *gorm.DB) error {
    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    
    sqlDB, err := db.DB()
    if err != nil {
        return fmt.Errorf("failed to get database: %w", err)
    }
    
    err = sqlDB.Close()
    if err != nil {
        return fmt.Errorf("failed to close database: %w", err)
    }
    return nil
}

func addDithered(db *gorm.DB, uuid string, palette string, ditherAlgorithm string, ditherStrength float64) error {
    type DitheredImage struct {
        ID              uint `gorm:"primarykey"`
        UUID            string `gorm:"not null"`
        Palette         string `gorm:"not null"`
        DitherAlgorithm string `gorm:"not null"`
        DitherStrength  float64 `gorm:"not null;default:1.0"`
        Path            string `gorm:"uniqueIndex;not null"`
        CreatedAt       time.Time
        UpdatedAt       time.Time
    }
    
    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    
    // Check if dithered image exists
    var count int64
    db.Model(&DitheredImage{}).Where("uuid = ? AND palette = ? AND dither_algorithm = ? AND dither_strength = ?", 
        uuid, palette, ditherAlgorithm, ditherStrength).Count(&count)
    
    if count > 0 {
        return nil // Already exists
    }
    
    // Generate path for dithered image
    path := fmt.Sprintf("dithered/%s_%s_%s_%0.2f.png", uuid, palette, ditherAlgorithm, ditherStrength)
    
    // Create new dithered image
    dithered := DitheredImage{
        UUID:            uuid,
        Palette:         palette,
        DitherAlgorithm: ditherAlgorithm,
        DitherStrength:  ditherStrength,
        Path:            path,
    }
    
    result := db.Create(&dithered)
    if result.Error != nil {
        return fmt.Errorf("failed to insert dithered image into database: %w", result.Error)
    }
    
    log.Printf("Inserted dithered image with UUID: %s\n", uuid)
    return nil
}

func removeDithered(db *gorm.DB, uuid string, palette string, ditherAlgorithm string, ditherStrength float64) error {
    type DitheredImage struct {
        ID              uint `gorm:"primarykey"`
        UUID            string `gorm:"not null"`
        Palette         string `gorm:"not null"`
        DitherAlgorithm string `gorm:"not null"`
        DitherStrength  float64 `gorm:"not null;default:1.0"`
        Path            string `gorm:"uniqueIndex;not null"`
        CreatedAt       time.Time
        UpdatedAt       time.Time
    }
    
    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    
    result := db.Where("uuid = ? AND palette = ? AND dither_algorithm = ? AND dither_strength = ?", 
        uuid, palette, ditherAlgorithm, ditherStrength).Delete(&DitheredImage{})
    
    if result.Error != nil {
        return fmt.Errorf("failed to delete dithered image from database: %w", result.Error)
    }
    
    log.Printf("Deleted dithered image with UUID: %s\n", uuid)
    return nil
}

func createRandomList(db *gorm.DB) error {
    type Image struct {
        UUID string
    }
    
    type RandomImage struct {
        ID   uint `gorm:"primarykey"`
        UUID string `gorm:"uniqueIndex;not null"`
    }
    
    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    
    // Clear existing random images
    db.Exec("DELETE FROM random_images")
    
    // Get all image UUIDs
    var images []Image
    if err := db.Model(&Image{}).Select("uuid").Find(&images).Error; err != nil {
        return fmt.Errorf("failed to fetch image UUIDs: %w", err)
    }
    
    // Shuffle the UUIDs
    rand.Seed(time.Now().UnixNano())
    rand.Shuffle(len(images), func(i, j int) {
        images[i], images[j] = images[j], images[i]
    })
    
    // Insert into random_images table
    for _, img := range images {
        db.Create(&RandomImage{UUID: img.UUID})
    }
    
    log.Println("Random images table created and populated with shuffled UUIDs from images table.")
    return nil
}

func startAPIServer(db *gorm.DB) {
    router := gin.Default()

    // Use closures to pass the db connection to handlers
    router.GET("/register", func(c *gin.Context) {
        handleRegisterRequest(c, db)
    })

    router.GET("/dev", func(c *gin.Context) {
        handleDeviceRequest(c, db)
    })

    router.GET("/admin/device_register", func(c *gin.Context) {
        // Admin endpoint, can be used for management tasks
        handleAdminDeviceRegisterRequest(c, db)
    })

    
    log.Println("Starting API server on port 8080...")
    log.Fatal(router.Run(":8080"))
}

func main(){
    db, err := dbInit()
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer dbClose(db)

    if err := refreshImages(db); err != nil {
        log.Fatalf("Failed to refresh images: %v", err)
    }

    if err := createRandomList(db); err != nil {
        log.Fatalf("Failed to create random image list: %v", err)
    }

    // Start API server
    startAPIServer(db)
}
