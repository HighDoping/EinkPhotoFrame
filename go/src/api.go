package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var jwtMasterKey []byte
var adminKey string
var imageDir string
var imageDirRefresh int
var cacheDir string
// Define models
type Device struct {
    ID         uint `gorm:"primarykey"`
    DeviceID   string `gorm:"uniqueIndex;not null"`
    DeviceName string `gorm:"not null"`
    DeviceToken string `gorm:"unique"`
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
    DitherStrength  float32 `gorm:"not null;default:1.0"`
    ResizeMethod    string `gorm:"not null;default:'cut'"`
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

type DBImage struct {
    ID        uint `gorm:"primarykey"`
    Path      string `gorm:"uniqueIndex;not null"`
    UUID      string `gorm:"uniqueIndex;not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DitheredImages []DitheredImage `gorm:"foreignKey:DBImageUUID;references:UUID"`
}

type DitheredImage struct {
    ID              uint `gorm:"primarykey"`
    UUID            string `gorm:"not null"`
    DBImageUUID     string `gorm:"not null"` // Foreign key to DBImage
    Palette         string `gorm:"not null"`
    DitherAlgorithm string `gorm:"not null"`
    DitherStrength  float32 `gorm:"not null;default:1.0"`
    Height          int `gorm:"not null;default:480"`
    Width           int `gorm:"not null;default:800"`
    ResizeMethod    string `gorm:"not null;default:'cut'"`
    Path            string `gorm:"uniqueIndex;not null"`
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type RandomImage struct {
    ID   uint `gorm:"primarykey"`
    UUID string `gorm:"uniqueIndex;not null"`
}

    
func init() {
    // Load .env file
    err := godotenv.Load()
    if err != nil {
        log.Println("Warning: Error loading .env file:", err)
    }

    // Get admin key from environment
    adminKey= os.Getenv("ADMIN_KEY")
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
    imageDirRefresh,err= strconv.Atoi(os.Getenv("IMAGE_DIR_REFRESH"))
    if err!=nil{
        log.Println("Warning: IMAGE_DIR_REFRESH not set in .env, using default")
        imageDirRefresh=86400
    }
    cacheDir= os.Getenv("CACHE_DIR")
    if cacheDir == "" {
        log.Println("Warning: CACHE_DIR not set in .env, using default")
        cacheDir,_ = os.UserCacheDir()
    }

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
    deviceName, ok := requestData["device_name"].(string)
    if !ok || deviceName == "" {
        deviceName = deviceID // Use device_id as default name if not provided
    }
    // Check if device already exists
    var existingDevice Device
    result := db.Where(&Device{DeviceID: deviceID,DeviceName: deviceName}).First(&existingDevice)
    if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
        log.Printf("Error checking existing device: %v", result.Error)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return result.Error
    }
    if result.RowsAffected > 0 {
        // Device already exists, check if has settings, if not create default settings
        var settings DeviceSetting
        result = db.Where(&DeviceSetting{DeviceID: deviceID}).First(&settings)
        if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
            log.Printf("Error checking device settings: %v", result.Error)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return result.Error
        }
        if result.RowsAffected == 0 {
            // Create default settings
            settings = DeviceSetting{
                DeviceID: deviceID,
                Device: existingDevice,}
            result = db.Create(&settings)
            if result.Error != nil {
                log.Printf("Error creating default settings: %v", result.Error)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
                return result.Error
            }
            log.Printf("Created default settings for device: %s", deviceID)
        }
        //create bearer token
        tokenString:=generateUUID()
        //save to device table
        existingDevice.DeviceToken = tokenString
        existingDevice.UpdatedAt = time.Now()
        result = db.Save(&existingDevice)
        if result.Error != nil {
            log.Printf("Error saving device: %v", result.Error)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return result.Error
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
    device_name := requestData["device_name"].(string)
    if device_id == "" || device_name == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "device_id and device_name are required"})
        return
    }
    // Insert the device into the database
    device := Device{
        DeviceID:   device_id,
        DeviceName: device_name,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    result := db.Create(&device)
    if result.Error != nil {
        log.Printf("Error inserting device: %v", result.Error)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Device registered successfully", "device_id": device.DeviceID, "device_name": device.DeviceName})
    log.Printf("Device registered: %s (%s)", device.DeviceID, device.DeviceName)
}

func checkAdminKey(c *gin.Context) bool {
    // Check if the request has a valid admin key
    tokenString := c.GetHeader("Authorization")
    if tokenString == "" {
        return false
    }
    tokenString = tokenString[len("Bearer "):] // Remove "Bearer " prefix
    return  tokenString == adminKey
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


func authDevice(c *gin.Context, db *gorm.DB) (Device, error) {
    // Check if the request has a valid device token, returns device details
    deviceToken,err:=getBearerToken(c)
    if err != nil {
        log.Printf("Error getting Bearer token: %v", err)
        return Device{}, err
    }

    // Fetch device details from the database
    var device Device
    result := db.Where(&Device{DeviceToken: deviceToken}).First(&device)
    if result.Error != nil {
        log.Printf("Error fetching device: %v", result.Error)
        return Device{},  result.Error
    }
    if device.DeviceID == "" {
        return Device{},  fmt.Errorf("device not found")
    }
    log.Printf("Device authenticated: %s (%s)", device.DeviceID, device.DeviceName)
    // Update last seen timestamp for the device
    err = updateLastSeen(device, db)
    if err != nil {
        log.Printf("Error updating last seen for device %s: %v", device.DeviceID, err)
        return Device{},  err
    }
    // Return device details and claims
    return device, nil
}

func handleDeviceRequest(c *gin.Context, db *gorm.DB) {
    device, err := authDevice(c, db)
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
    if requestData["action"] == "get_settings" {
        // Get device settings
        var settings DeviceSetting
        result := db.Where(&DeviceSetting{DeviceID:  device.DeviceID}).First(&settings)
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
        result := db.Where(&DeviceSetting{DeviceID:  device.DeviceID}).First(&settings)
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
        if ditherStrength, ok := requestData["dither_strength"].(float32); ok {
            settings.DitherStrength = ditherStrength
        }
        if resizeMethod, ok := requestData["resize_method"].(string); ok {
            settings.ResizeMethod = resizeMethod
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
        result := db.Where(&DeviceTelemetry{DeviceID: device.DeviceID}).First(&telemetry)
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
        result := db.Where(&DeviceSetting{DeviceID:  device.DeviceID}).First(&settings)
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
        }     
        nextImage,err:=getNextRandom(db,device)
        if err != nil{
            log.Printf("Error finding next random image:%v",err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        ditheredImage,err:=getDithered(db, nextImage, settings.Palette, settings.DitherAlgorithm, settings.DitherStrength,settings.Width,settings.Height,settings.ResizeMethod)
        if err != nil {
            log.Printf("Error getting dithered image: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        ditheredImg,err:=loadImage(ditheredImage.Path)
        if err != nil {
            log.Printf("Error loading dithered image: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        ditheredImgBit:=imgToBitmap(ditheredImg,settings.Palette, settings.Width, settings.Height)
        // Update device's current image
        device.CurrentImage = nextImage.UUID
        device.UpdatedAt = time.Now()
        db.Save(&device)
    
        // Return the processed image or image data
        c.JSON(http.StatusOK, gin.H{
            "message": "Image updated", 
            "image_uuid": nextImage.UUID,
            "image": ditheredImgBit,
        })
    }
    if requestData["action"] == "update_image" {
        // Force update image without time check
        var settings DeviceSetting
        result := db.Where(&DeviceSetting{DeviceID:  device.DeviceID}).First(&settings)
        if result.Error != nil {
            if result.Error == gorm.ErrRecordNotFound {
                c.JSON(http.StatusNotFound, gin.H{"error": "Settings not found"})
            } else {
                log.Printf("Error fetching settings: %v", result.Error)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            }
            return
        }
        
        nextImage, err := getNextRandom(db, device)
        if err != nil {
            log.Printf("Error finding next random image: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        
        ditheredImage, err := getDithered(db, nextImage, settings.Palette, settings.DitherAlgorithm, 
            settings.DitherStrength, settings.Width, settings.Height, settings.ResizeMethod)
        if err != nil {
            log.Printf("Error getting dithered image: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        
        ditheredImg, err := loadImage(ditheredImage.Path)
        if err != nil {
            log.Printf("Error loading dithered image: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
            return
        }
        
        ditheredImgBit := imgToBitmap(ditheredImg, settings.Palette, settings.Width, settings.Height)
        //save dithered image to cache
        print(len(ditheredImgBit))
        filepaths := make([]string, len(ditheredImgBit))
        for i:=0;i<len(ditheredImgBit);i++ {
            bytes_data:=BitsToBytes(ditheredImgBit[i])
            filePath := fmt.Sprintf("%s/%s_%d.bin", cacheDir, ditheredImage.UUID,i)
            err = saveBytesToFile(filePath, bytes_data)
            if err != nil {
                log.Printf("Error saving dithered image to file: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
                return
            }
            filepaths[i] = strings.Replace(filePath, cacheDir, "assets", 1)
        }
        // Update device's current image
        device.CurrentImage = nextImage.UUID
        device.UpdatedAt = time.Now()
        db.Save(&device)
        
        c.JSON(http.StatusOK, gin.H{
            "message": "Image updated", 
            "image_uuid": nextImage.UUID,
            "image": filepaths,
        })
        return
    }
}

func getNextRandom(db *gorm.DB, device Device) (DBImage, error) {
    var nextImage DBImage
    
    if device.CurrentImage == "" {
        // First time, get first random image
        var firstRandom RandomImage
        if err := db.Order("id ASC").First(&firstRandom).Error; err != nil {
            return nextImage, fmt.Errorf("no images available")
        }
        
        if err := db.Where(&DBImage{UUID: firstRandom.UUID}).First(&nextImage).Error; err != nil {
            return nextImage, fmt.Errorf("failed to find image with UUID: %s", firstRandom.UUID)
        }
    } else {
        // Find current position and get next
        var currentRandom RandomImage
        result := db.Where(&RandomImage{UUID:device.CurrentImage}).First(&currentRandom)
        
        if result.Error != nil {
            // Current image not in random list, start from beginning
            if err := db.Order("id ASC").First(&currentRandom).Error; err != nil {
                return nextImage, fmt.Errorf("no images available")
            }
        } else {
            // Get next random image
            var nextRandom RandomImage
            result := db.Where("id > ?", currentRandom.ID).Order("id ASC").First(&nextRandom)
            
            if result.Error != nil {
                // Wrap around to first
                if result.Error == gorm.ErrRecordNotFound {
                    // Handle wrap-around case - get the first record
                    if err := db.Order("id ASC").First(&nextRandom).Error; err != nil {
                        return nextImage, fmt.Errorf("no images available")
                    }
                    currentRandom = nextRandom
                } else {
                    // Other database error
                    return nextImage, fmt.Errorf("database error: %w", result.Error)
                }
            } else {
                currentRandom = nextRandom
            }
        }
        
        if err := db.Where(DBImage{UUID:currentRandom.UUID}).First(&nextImage).Error; err != nil {
            return nextImage, fmt.Errorf("failed to find image with UUID: %s", currentRandom.UUID)
        }
    }
    
    return nextImage, nil
}


func dbInit() (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open("./db.db"), &gorm.Config{})
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    // Auto migrate schemas
    err = db.AutoMigrate(&Device{}, &DeviceSetting{}, &DeviceTelemetry{}, &DBImage{}, &DitheredImage{}, &RandomImage{})
    if err != nil {
        return nil, fmt.Errorf("failed to migrate database schema: %w", err)
    }

    // Clean DitheredImage table
    var ditheredImages []DitheredImage
    if err := db.Find(&ditheredImages).Error; err != nil {
        return nil, fmt.Errorf("failed to fetch dithered images: %w", err)
    }
    for _, dithered := range ditheredImages {
        // Check if the dithered image file exists
        if err := db.Delete(&dithered).Error; err != nil {
            log.Printf("failed to delete dithered image %s from database: %v\n", dithered.Path, err)
            continue
        }
        log.Printf("Deleted dithered image: %s with UUID: %s (file no longer exists)\n", dithered.Path, dithered.UUID)
    }

    return db, nil
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
func refreshImages(db *gorm.DB) error {
    imagePaths, err := generateFileList(imageDir, []string{".jpg", ".jpeg", ".png", ".bmp"})
    if err != nil {
        return fmt.Errorf("failed to generate file list: %w", err)
    }

    // Create a map for quick lookup of paths
    imagePathMap := make(map[string]bool)
    for _, path := range imagePaths {
        imagePathMap[path] = true
    }

    // Find and remove entries in DB that no longer exist in the file system
    var images []DBImage
    if err := db.Find(&images).Error; err != nil {
        return fmt.Errorf("failed to fetch existing images: %w", err)
    }

    for _, img := range images {
        if !imagePathMap[img.Path] {
            // File doesn't exist anymore, delete from database
            if err := db.Delete(&img).Error; err != nil {
                log.Printf("failed to delete non-existent image %s from database: %v\n", img.Path, err)
                continue
            }
            // Also clean up any dithered versions
            if err := db.Where("uuid = ?", img.UUID).Delete(&DitheredImage{}).Error; err != nil {
                log.Printf("failed to delete dithered images for %s: %v\n", img.UUID, err)
            }
            log.Printf("Deleted image: %s with UUID: %s (file no longer exists)\n", img.Path, img.UUID)
        }
    }

    // Add new images
    for _, path := range imagePaths {
        // Check if image already exists
        var count int64
        db.Model(&DBImage{}).Where("path = ?", path).Count(&count)
        if count > 0 {
            continue // Skip if exists
        }
        
        // Create new image
        uuid := generateUUID()
        image := DBImage{
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



func addDithered(db *gorm.DB, image DBImage, palette string, ditherAlgorithm string, ditherStrength float32,targetWidth int, targetHeight int,resizeMethod string) (DitheredImage, error) {
    if db == nil {
        return DitheredImage{},fmt.Errorf("database connection is nil")
    }
    // Generate path for dithered image
    uuid:=generateUUID()
    path := fmt.Sprintf("%s/dithered_%s.png", cacheDir,uuid)
    img:=fetchAndDither(image.Path, palette, ditherAlgorithm, ditherStrength, targetWidth, targetHeight,resizeMethod)
    if img == nil {
        return DitheredImage{},fmt.Errorf("failed to dither image: %s", image.Path)
    }
    // Save dithered image to cache
    err := saveImage( path,img)
    if err != nil {
        return DitheredImage{},fmt.Errorf("failed to save dithered image to file: %w", err)
    }
    
    dithered := DitheredImage{
        UUID:            uuid,
        DBImageUUID:     image.UUID,
        Palette:         palette,
        DitherAlgorithm: ditherAlgorithm,
        DitherStrength:  ditherStrength,
        Height:          targetHeight,
        Width:           targetWidth,
        ResizeMethod:    resizeMethod,
        CreatedAt:       time.Now(),
        UpdatedAt:       time.Now(),
        Path:            path,
    }
    
    result := db.Create(&dithered)
    if result.Error != nil {
        return DitheredImage{},fmt.Errorf("failed to insert dithered image into database: %w", result.Error)
    }

    log.Printf("Inserted dithered image with UUID: %s\n", dithered.UUID)
    return dithered, nil
}

func getDithered(db *gorm.DB, image DBImage, palette string, ditherAlgorithm string, ditherStrength float32,targetWidth int, targetHeight int,resizeMethod string) (DitheredImage, error) {
    if db == nil {
        return DitheredImage{}, fmt.Errorf("database connection is nil")
    }
    
    // Check if dithered image already exists
    var dithered DitheredImage
    result := db.Where(&DitheredImage{
        DBImageUUID:     image.UUID,
        Palette:         palette,
        DitherAlgorithm: ditherAlgorithm,
        DitherStrength:  ditherStrength,
        Width:           targetWidth,
        Height:          targetHeight,
        ResizeMethod:    resizeMethod,
    }).First(&dithered)

    // If not found, create it
    if result.Error != nil {
        if result.Error == gorm.ErrRecordNotFound {
            // Dithered image not found, create it
            dithered, err := addDithered(db, image, palette, ditherAlgorithm, ditherStrength, targetWidth,targetHeight,resizeMethod)
            if err != nil {
                return DitheredImage{}, fmt.Errorf("failed to create dithered image: %w", err)
            }
            return dithered, nil}
        
        return DitheredImage{}, fmt.Errorf("failed to query dithered image: %w", result.Error)}
    // return found dithered image
    return dithered, nil
}

func removeDithered(db *gorm.DB, uuid string, palette string, ditherAlgorithm string, ditherStrength float32,targetWidth int, targetHeight int,resizeMethod string) error {    
    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    
    // First get the dithered image path
    var dithered DitheredImage
    if err := db.Where(&DitheredImage{UUID: uuid, Palette: palette, DitherAlgorithm: ditherAlgorithm, DitherStrength: ditherStrength, Width: targetWidth, Height: targetHeight, ResizeMethod: resizeMethod}).First(&dithered).Error; err != nil {
        return fmt.Errorf("failed to find dithered image: %w", err)
    }
    
    // Delete from database
    result := db.Where(&DitheredImage{UUID: uuid, Palette: palette, DitherAlgorithm: ditherAlgorithm, DitherStrength: ditherStrength, Width: targetWidth, Height: targetHeight, ResizeMethod: resizeMethod}).Delete(&DitheredImage{})
    if result.Error != nil {
        return fmt.Errorf("failed to delete dithered image from database: %w", result.Error)
    }
    
    // Delete the image file from cache
    if err := os.Remove(dithered.Path); err != nil {
        log.Printf("Warning: Failed to delete cached file %s: %v", dithered.Path, err)
        // Continue anyway as the database record is deleted
    }
    
    log.Printf("Deleted dithered image with UUID: %s\n", uuid)
    return nil
}

func createRandomList(db *gorm.DB) error {

    if db == nil {
        return fmt.Errorf("database connection is nil")
    }
    
    // Clear existing random images
    db.Exec("DELETE FROM random_images")
    
    // Get all image UUIDs
    var images []DBImage
    if err := db.Model(&DBImage{}).Select("uuid").Find(&images).Error; err != nil {
        return fmt.Errorf("failed to fetch image UUIDs: %w", err)
    }
    
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

func updateRandomList(db *gorm.DB) error {
    // For each entry in random_images, check if the corresponding image exists in images
    // delete the entry if the image does not exist
    // Then check images and add new random images if needed
    var randomImages []RandomImage
    if err := db.Find(&randomImages).Error; err != nil {
        return fmt.Errorf("failed to fetch random images: %w", err)
    }
    for _, randomImage := range randomImages {
        var count int64
        db.Model(&DBImage{}).Where("uuid = ?", randomImage.UUID).Count(&count)
        if count == 0 {
            // DBImage does not exist, delete from random_images
            if err := db.Delete(&RandomImage{}, randomImage.ID).Error; err != nil {
                return fmt.Errorf("failed to delete random image %s: %w", randomImage.UUID, err)
            }
            log.Printf("Deleted random image with UUID: %s\n", randomImage.UUID)
        }
    }
    // Now check if we need to add new random images
    var images []DBImage
    if err := db.Model(&DBImage{}).Select("uuid").Find(&images).Error; err != nil {
        return fmt.Errorf("failed to fetch image UUIDs: %w", err)
    }
    // Get current random images
    // Get the current count of random images
    var randomCount int64
    if err := db.Model(&RandomImage{}).Count(&randomCount).Error; err != nil {
        return fmt.Errorf("failed to count random images: %w", err)
    }

    // For each image that isn't in the random list, insert it at a random position
    for _, img := range images {
        var count int64
        db.Model(&RandomImage{}).Where("uuid = ?", img.UUID).Count(&count)
        if count == 0 {
            // DBImage does not exist in random_images, add it at a random position
            position := 0
            
            // Only calculate random position if we have existing entries
            if randomCount > 0 {
                position = rand.Intn(int(randomCount))
            }
            
            // Shift all entries at or after the random position, starting from the highest ID
            // This prevents UNIQUE constraint violations
            var maxID uint
            if randomCount > 0 {
                if err := db.Model(&RandomImage{}).Select("MAX(id)").Scan(&maxID).Error; err != nil {
                    return fmt.Errorf("failed to get max ID: %w", err)
                }
                
                // Update IDs from highest to lowest to avoid conflicts
                for i := maxID; i >= uint(position+1); i-- {
                    if err := db.Exec("UPDATE random_images SET id = ? WHERE id = ?", i+1, i).Error; err != nil {
                        return fmt.Errorf("failed to shift random image with ID %d: %w", i, err)
                    }
                }
            }
            
            // Insert the new entry at the random position
            randomImage := RandomImage{ID: uint(position + 1), UUID: img.UUID}
            if err := db.Create(&randomImage).Error; err != nil {
                return fmt.Errorf("failed to insert random image %s at position %d: %w", img.UUID, position, err)
            }
            
            log.Printf("Inserted random image with UUID: %s at position %d\n", img.UUID, position)
            randomCount++ // Update the count for next iteration
        }
    }
    log.Println("Random images list updated.")
    return nil
}

func startAPIServer(db *gorm.DB) {
    router := gin.Default()

    // Use closures to pass the db connection to handlers
    // Serve static files with authentication
    router.GET("/assets/*filepath", func(c *gin.Context) {
        // Check authentication first
        device, err := authDevice(c, db)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized access to assets"})
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
    log.Fatal(router.RunTLS(":8080","cert.pem","key.pem"))
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

    if err := updateRandomList(db); err != nil {
        log.Fatalf("Failed to update random image list: %v", err)
    }

    // Start API server
    startAPIServer(db)
}
