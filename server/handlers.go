package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/HighDoping/EinkPhotoFrame/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Helper functions for standardized responses
func successResponse(data interface{}) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
	}
}

func errorResponse(message string) APIResponse {
	return APIResponse{
		Success: false,
		Error:   message,
	}
}

func handleRegisterRequest(c *gin.Context, db *gorm.DB) error {
	// Enroll a device that was explicitly pre-registered by an administrator.
	var requestData map[string]interface{}
	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("Invalid JSON"))
		return err
	}
	deviceID, ok := requestData["device_id"].(string)
	if !ok || deviceID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("device_id is required"))
		return fmt.Errorf("device_id is required")
	}
	deviceName, ok := requestData["device_name"].(string)
	if !ok || strings.TrimSpace(deviceName) == "" {
		c.JSON(http.StatusBadRequest, errorResponse("device_name is required"))
		return fmt.Errorf("device_name is required")
	}
	deviceID = strings.TrimSpace(deviceID)
	deviceName = strings.TrimSpace(deviceName)
	width, widthOK := requestData["width"].(float64)
	height, heightOK := requestData["height"].(float64)
	if !widthOK || !heightOK || width < 1 || height < 1 || width != float64(int(width)) || height != float64(int(height)) {
		c.JSON(http.StatusBadRequest, errorResponse("valid width and height are required"))
		return fmt.Errorf("invalid device dimensions")
	}
	// Check if device already exists
	var existingDevice Device
	// Device names are managed by the administrator and may differ from the
	// firmware's configured name. Authorization is based on the immutable ID.
	result := db.Where(&Device{DeviceID: deviceID}).First(&existingDevice)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		log.Printf("Error checking existing device: %v", result.Error)
		c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
		return result.Error
	}
	if result.RowsAffected > 0 {
		if !existingDevice.Enabled {
			c.JSON(http.StatusUnauthorized, errorResponse("Device enrollment is disabled"))
			return fmt.Errorf("device enrollment disabled")
		}
		if existingDevice.DeviceName != deviceName {
			c.JSON(http.StatusUnauthorized, errorResponse("Unauthorized device registration"))
			log.Printf("Registration name mismatch for device: %s", deviceID)
			return fmt.Errorf("device name mismatch")
		}
		// Device already exists, check if has settings, if not create default settings
		var settings DeviceSetting
		result = db.Where(&DeviceSetting{DeviceID: deviceID}).First(&settings)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			log.Printf("Error checking device settings: %v", result.Error)
			c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
			return result.Error
		}
		if result.RowsAffected == 0 {
			// Create default settings
			settings = DeviceSetting{
				DeviceID: deviceID,
				Device:   existingDevice,
			}
			result = db.Create(&settings)
			if result.Error != nil {
				log.Printf("Error creating default settings: %v", result.Error)
				c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
				return result.Error
			}
			log.Printf("Created default settings for device: %s", deviceID)
		}
		// Dimensions come from the device firmware and are not admin-controlled.
		settings.Width = int(width)
		settings.Height = int(height)
		if err := db.Save(&settings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("Unable to save device dimensions"))
			return err
		}
		//create bearer token
		tokenString := generateUUID()
		//save to device table
		existingDevice.DeviceToken = tokenString
		existingDevice.UpdatedAt = time.Now()
		result = db.Save(&existingDevice)
		if result.Error != nil {
			log.Printf("Error saving device: %v", result.Error)
			c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
			return result.Error
		}

		c.JSON(http.StatusOK, successResponse(map[string]interface{}{
			"message": "Device registered",
			"token":   tokenString,
		}))
		log.Printf("Device registered: %s (%s)", existingDevice.DeviceID, existingDevice.DeviceName)
		return nil
	}
	// Device does not exist, deny registration
	c.JSON(http.StatusUnauthorized, errorResponse("Unauthorized device registration"))
	log.Printf("Unauthorized device registration attempt: %s", deviceID)
	return fmt.Errorf("unauthorized device registration")
}
func handleAdminDeviceRegisterRequest(c *gin.Context, db *gorm.DB, cfg config.Config) {
	// Check if the request has a valid admin key
	if !checkAdminKey(c, cfg) {
		c.JSON(http.StatusUnauthorized, errorResponse("Unauthorized access"))
		return // Unauthorized access
	}
	var requestData createDeviceRequest
	if err := c.ShouldBindJSON(&requestData); err != nil || strings.TrimSpace(requestData.DeviceID) == "" || strings.TrimSpace(requestData.DeviceName) == "" {
		c.JSON(http.StatusBadRequest, errorResponse("device_id and device_name are required"))
		return
	}
	device_id := strings.TrimSpace(requestData.DeviceID)
	device_name := strings.TrimSpace(requestData.DeviceName)

	// Check if the device_id already exists in the database
	var existingDevice Device
	result := db.Where("device_id = ?", device_id).First(&existingDevice)
	if result.Error == nil {
		// Device already exists
		c.JSON(http.StatusConflict, errorResponse("Device with this device_id already exists, name: "+existingDevice.DeviceName))
		return
	} else if result.Error != gorm.ErrRecordNotFound {
		// Some other database error occurred
		log.Printf("Error checking existing device: %v", result.Error)
		c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
		return
	}

	// Insert the device into the database
	device := Device{
		DeviceID:   device_id,
		DeviceName: device_name,
		Enabled:    true,
		StartIndex: randomStartIndex(db),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	result = db.Create(&device)
	if result.Error != nil {
		log.Printf("Error inserting device: %v", result.Error)
		c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
		return
	}
	c.JSON(http.StatusOK, successResponse(map[string]interface{}{
		"message":     "Device registered successfully",
		"device_id":   device.DeviceID,
		"device_name": device.DeviceName,
	}))
	log.Printf("Device registered: %s (%s)", device.DeviceID, device.DeviceName)
}

func handleDeviceRequest(c *gin.Context, db *gorm.DB, cfg config.Config) {
	device, err := authDevice(c, db)
	if err != nil {
		c.JSON(http.StatusUnauthorized, errorResponse("Unauthorized device"))
		return
	}
	//get json body
	var requestData map[string]interface{}
	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("Invalid JSON"))
		return
	}
	if requestData["action"] == "get_settings" {
		// Get device settings
		var settings DeviceSetting
		result := db.Where(&DeviceSetting{DeviceID: device.DeviceID}).First(&settings)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, errorResponse("Settings not found"))
			} else {
				log.Printf("Error fetching settings: %v", result.Error)
				c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
			}
			return
		}
		c.JSON(http.StatusOK, successResponse(map[string]interface{}{
			"settings": settings,
		}))
		return
	}
	if requestData["action"] == "update_settings" {
		// Update device settings
		var settings DeviceSetting
		result := db.Where(&DeviceSetting{DeviceID: device.DeviceID}).First(&settings)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				// Create new settings if not found
				settings = DeviceSetting{
					DeviceID:  device.DeviceID,
					CreatedAt: time.Now(),
				}
			} else {
				log.Printf("Error fetching settings: %v", result.Error)
				c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
				return
			}
		}
		// Update settings with request data
		if imgUpdateInterval, ok := requestData["img_update_interval"].(int); ok {
			settings.ImgUpdateInterval = imgUpdateInterval
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
			c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
			return
		}

		c.JSON(http.StatusOK, successResponse(map[string]interface{}{
			"message":  "Settings updated successfully",
			"settings": settings,
		}))
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
				c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
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
			c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
			return
		}

		c.JSON(http.StatusOK, successResponse(map[string]interface{}{
			"message":   "Telemetry updated successfully",
			"telemetry": telemetry,
		}))
		return
	}
	if requestData["action"] == "get_image" {
		imageUpdate, err := processDeviceImageUpdate(db, cfg, &device, false)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, errorResponse("Settings not found"))
			} else {
				log.Printf("Error processing image update: %v", err)
				c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
			}
			return
		}
		if imageUpdate.NoUpdate {
			c.JSON(http.StatusOK, successResponse(map[string]interface{}{
				"message": "No image update needed",
			}))
			return
		}

		c.JSON(http.StatusOK, successResponse(map[string]interface{}{
			"message":    "Image updated",
			"image_uuid": imageUpdate.ImageUUID,
			"image":      imageUpdate.Files,
		}))
		return
	}
	if requestData["action"] == "update_image" {
		imageUpdate, err := processDeviceImageUpdate(db, cfg, &device, true)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, errorResponse("Settings not found"))
			} else {
				log.Printf("Error processing image update: %v", err)
				c.JSON(http.StatusInternalServerError, errorResponse("Internal server error"))
			}
			return
		}

		c.JSON(http.StatusOK, successResponse(map[string]interface{}{
			"message":    "Image updated",
			"image_uuid": imageUpdate.ImageUUID,
			"image":      imageUpdate.Files,
		}))
		return
	}

	// If no valid action was specified
	c.JSON(http.StatusBadRequest, errorResponse("Invalid or missing action"))
}
