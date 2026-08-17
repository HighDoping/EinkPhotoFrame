package main

import (
	"crypto/subtle"
	"fmt"
	"log"
	"time"

	"github.com/HighDoping/EinkPhotoFrame/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func checkAdminKey(c *gin.Context, cfg config.Config) bool {
	// Check if the request has a valid admin key.
	tokenString, err := getBearerToken(c)
	return err == nil && cfg.AdminKey != "" && subtle.ConstantTimeCompare([]byte(tokenString), []byte(cfg.AdminKey)) == 1
}

func updateLastSeen(device Device, db *gorm.DB) error {
	// Maintain one current status row per device.
	deviceTelemetry := DeviceTelemetry{DeviceID: device.DeviceID}
	result := db.Where("device_id = ?", device.DeviceID).First(&deviceTelemetry)
	if result.Error == gorm.ErrRecordNotFound {
		deviceTelemetry.LastSeen = time.Now()
		result = db.Create(&deviceTelemetry)
	} else if result.Error == nil {
		deviceTelemetry.LastSeen = time.Now()
		result = db.Save(&deviceTelemetry)
	}
	if result.Error != nil {
		log.Printf("Error updating last seen for device %s: %v", device.DeviceID, result.Error)
		return result.Error
	}
	log.Printf("Last seen updated for device %s at %v", device.DeviceID, deviceTelemetry.LastSeen)
	return nil
}

func getBearerToken(c *gin.Context) (string, error) {
	// Extract the Bearer token from the Authorization header.
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
	// Check if the request has a valid device token, returns device details.
	deviceToken, err := getBearerToken(c)
	if err != nil {
		log.Printf("Error getting Bearer token: %v", err)
		return Device{}, err
	}

	// Fetch device details from the database.
	var device Device
	result := db.Where(&Device{DeviceToken: deviceToken}).First(&device)
	if result.Error != nil {
		log.Printf("Error fetching device: %v", result.Error)
		return Device{}, result.Error
	}
	if device.DeviceID == "" {
		return Device{}, fmt.Errorf("device not found")
	}
	if !device.Enabled {
		return Device{}, fmt.Errorf("device is disabled")
	}
	log.Printf("Device authenticated: %s (%s)", device.DeviceID, device.DeviceName)

	err = updateLastSeen(device, db)
	if err != nil {
		log.Printf("Error updating last seen for device %s: %v", device.DeviceID, err)
		return Device{}, err
	}

	return device, nil
}
