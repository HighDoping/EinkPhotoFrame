package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func startAPIServer(db *sql.DB) {
    router := gin.Default()

    // Use closures to pass the db connection to handlers
    router.GET("/settings", func(c *gin.Context) {
        handleSettingsRequest(c, db)
    })
    
    router.GET("/bitmap", func(c *gin.Context) {
        handleBitmapRequest(c, db)
    })

    router.GET("/image", func(c *gin.Context) {
        serveImage(c, db)
    })
    router.GET("/dithered", func(c *gin.Context) {
        serveDitheredImage(c, db)
    })
    
    log.Println("Starting API server on port 8080...")
    log.Fatal(router.Run(":8080"))
}

func dbInit() (*sql.DB, error) {
    db, err := sql.Open("sqlite3", "./settings.db")
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    // Create table if it doesn't exist
    createDeviceSettingsTableSQL := `CREATE TABLE IF NOT EXISTS settings (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        device_id TEXT NOT NULL,
        setting JSON NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
    );`
    _, err = db.Exec(createDeviceSettingsTableSQL)
    if err != nil {
        return nil, fmt.Errorf("failed to create table: %w", err)
    }
    return db, nil
}
func handleSettingsRequest(c *gin.Context,db *sql.DB) {
    authHeader := c.GetHeader("Authorization")
    if authHeader == "" || len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
        return
    }
    
    deviceID := authHeader[7:] // Extract the token after "Bearer "
    if deviceID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID token"})
        return
    }

    var setting string
    err := db.QueryRow("SELECT setting FROM settings WHERE device_id = ?", deviceID).Scan(&setting)
    if err != nil {
        if err == sql.ErrNoRows {
            c.JSON(http.StatusNotFound, gin.H{"error": "Device ID not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Database error: %v", err)})
        return
    }
    // Return the setting as JSON
    
    c.JSON(http.StatusOK, gin.H{"device_id": deviceID, "setting": setting})
}
func handleBitmapRequest(c *gin.Context, db *sql.DB) {
    authHeader := c.GetHeader("Authorization")
    if authHeader == "" || len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
        return
    }

    deviceID := authHeader[7:] // Extract the token after "Bearer "
    if deviceID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID token"})
        return
    }

    var setting string
    err := db.QueryRow("SELECT setting FROM settings WHERE device_id = ?", deviceID).Scan(&setting)
    if err != nil {
        if err == sql.ErrNoRows {
            c.JSON(http.StatusNotFound, gin.H{"error": "Device ID not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Database error: %v", err)})
        return
    }

    // Parse the setting to get bitmap information
    // For simplicity, assuming setting is a JSON string with bitmap data
    // You should implement actual parsing logic based on your setting structure

    // Example response with dummy bitmap data
    bitmaps := []string{"bitmap1", "bitmap2"} // Replace with actual bitmap data

    c.JSON(http.StatusOK, gin.H{"device_id": deviceID, "bitmaps": bitmaps})}

func serveImage(c *gin.Context, db *sql.DB) {
    authHeader := c.GetHeader("Authorization")
    if authHeader == "" || len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
        return
    }

    deviceID := authHeader[7:] // Extract the token after "Bearer "
    if deviceID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID token"})
        return
    }

    // Here you would fetch the image based on the device ID
    // For simplicity, returning a dummy image URL
    imageURL := fmt.Sprintf("http://example.com/image/%s", deviceID)

    c.JSON(http.StatusOK, gin.H{"device_id": deviceID, "image_url": imageURL})
}

func serveDitheredImage(c *gin.Context, db *sql.DB) {
    authHeader := c.GetHeader("Authorization")
    if authHeader == "" || len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
        return
    }

    deviceID := authHeader[7:] // Extract the token after "Bearer "
    if deviceID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device ID token"})
        return
    }

    // Here you would fetch the dithered image based on the device ID
    // For simplicity, returning a dummy dithered image URL
    ditheredImageURL := fmt.Sprintf("http://example.com/dithered/%s", deviceID)

    c.JSON(http.StatusOK, gin.H{"device_id": deviceID, "dithered_image_url": ditheredImageURL})
}


func main() {
    //load the database
    db, err := dbInit()
    if err != nil {
        log.Fatalf("Error initializing database: %v", err)
    }
    defer db.Close()
    startAPIServer(db)
}


func createDitheredBitmaps() {
    folder:="../asset"
    file_ext := []string{".jpg", ".jpeg", ".png", ".bmp"}

    color_palette := "7Eink"
    dither_algorithm := "StevenPigeon"
    targetWidth := 800
    targetHeight := 480

    fileList, err := generateFileList(folder, file_ext)
    if err != nil {
        log.Println("Error generating file list:", err)
    }
    // Process each file
    for _, file := range fileList {
        img := fetchandDither(file, color_palette, dither_algorithm, targetWidth, targetHeight)
        if img == nil {
            log.Println("Skipping file due to error:", file)
            continue
        }
        // save the dithered image to a file
        outputFile := fmt.Sprintf("%s_dithered.png", file)
        err := saveImage(outputFile, img)
        if err != nil {
            log.Println("Error saving dithered image:", err)
            continue}
        // Convert the dithered image to a bitmap representation
        bitmaps:=imgToBitmap(img, color_palette, targetWidth, targetHeight)
        for i := range bitmaps {
            bytes:= BitsToBytes(bitmaps[i][:])

// save to .txt in 0x format
            outputFile = fmt.Sprintf("%s_%d.txt", file,i)
            err = saveBytesToFileHex(outputFile, bytes)
            if err != nil {
                log.Println("Error saving bytes to hex file:", err)
            } else {
                log.Println("Saved bitmap to hex file:", outputFile)
            }
        }
    }}