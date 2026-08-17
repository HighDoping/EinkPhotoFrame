package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/HighDoping/EinkPhotoFrame/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminDevice struct {
	DeviceID     string          `json:"device_id"`
	DeviceName   string          `json:"device_name"`
	Authorized   bool            `json:"authorized"`
	Enabled      bool            `json:"enabled"`
	StartIndex   uint            `json:"start_index"`
	CurrentImage *adminImage     `json:"current_image"`
	AssetCount   int64           `json:"asset_count"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Telemetry    DeviceTelemetry `json:"telemetry"`
	Settings     DeviceSetting   `json:"settings"`
}

type adminImage struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type createDeviceRequest struct {
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name" binding:"required"`
}

type updateDeviceRequest struct {
	DeviceName *string        `json:"device_name"`
	Settings   *DeviceSetting `json:"settings"`
}

func requireAdmin(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !checkAdminKey(c, cfg) {
			c.JSON(http.StatusUnauthorized, errorResponse("Unauthorized access"))
			c.Abort()
			return
		}
		c.Next()
	}
}

func handleAdminDevices(c *gin.Context, db *gorm.DB) {
	var devices []Device
	if err := db.Order("device_name, device_id").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Unable to load devices"))
		return
	}

	result := make([]adminDevice, 0, len(devices))
	for _, device := range devices {
		entry := adminDevice{DeviceID: device.DeviceID, DeviceName: device.DeviceName, Authorized: device.DeviceToken != "", Enabled: device.Enabled, StartIndex: device.StartIndex, CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt}
		if device.CurrentImage != "" {
			var current DBImage
			if err := db.Where("uuid = ?", device.CurrentImage).First(&current).Error; err == nil {
				entry.CurrentImage = &adminImage{UUID: current.UUID, Name: filepath.Base(current.Path)}
			}
		}
		db.Where("device_id = ?", device.DeviceID).First(&entry.Settings)
		db.Where("device_id = ?", device.DeviceID).First(&entry.Telemetry)
		db.Model(&DeviceImage{}).Where("device_id = ?", device.DeviceID).Count(&entry.AssetCount)
		result = append(result, entry)
	}
	c.JSON(http.StatusOK, successResponse(map[string]interface{}{"devices": result}))
}

func handleAdminCurrentImage(c *gin.Context, db *gorm.DB) {
	var device Device
	if err := db.Where("device_id = ?", c.Param("deviceID")).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, errorResponse("Device not found"))
		return
	}
	if device.CurrentImage == "" {
		c.JSON(http.StatusNotFound, errorResponse("No image is currently assigned"))
		return
	}
	var current DBImage
	if err := db.Where("uuid = ?", device.CurrentImage).First(&current).Error; err != nil {
		c.JSON(http.StatusNotFound, errorResponse("Current image is no longer available"))
		return
	}
	c.File(current.Path)
}

func handleAdminCreateDevice(c *gin.Context, db *gorm.DB) {
	var request createDeviceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("device_id and device_name are required"))
		return
	}
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	request.DeviceName = strings.TrimSpace(request.DeviceName)
	if request.DeviceID == "" || request.DeviceName == "" {
		c.JSON(http.StatusBadRequest, errorResponse("device_id and device_name are required"))
		return
	}

	device := Device{DeviceID: request.DeviceID, DeviceName: request.DeviceName, Enabled: true, StartIndex: randomStartIndex(db)}
	if err := db.Create(&device).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, errorResponse("A device with this device ID already exists"))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse("Unable to create device"))
		return
	}
	settings := DeviceSetting{DeviceID: device.DeviceID}
	if err := db.Create(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Device created but default settings could not be saved"))
		return
	}
	c.JSON(http.StatusCreated, successResponse(map[string]interface{}{"device": device, "settings": settings}))
}

func handleAdminUpdateDevice(c *gin.Context, db *gorm.DB) {
	var request updateDeviceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("Invalid JSON"))
		return
	}
	var device Device
	if err := db.Where("device_id = ?", c.Param("deviceID")).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, errorResponse("Device not found"))
		return
	}
	if request.DeviceName != nil {
		name := strings.TrimSpace(*request.DeviceName)
		if name == "" {
			c.JSON(http.StatusBadRequest, errorResponse("device_name cannot be empty"))
			return
		}
		device.DeviceName = name
		if err := db.Save(&device).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("Unable to save device"))
			return
		}
	}
	if request.Settings != nil {
		settings := *request.Settings
		settings.ID = 0
		settings.DeviceID = device.DeviceID
		settings.Device = Device{}
		var existing DeviceSetting
		if err := db.Where("device_id = ?", device.DeviceID).First(&existing).Error; err == nil {
			// Width and height are reported by the firmware at enrollment and must
			// not be changed through the server API.
			settings.Width = existing.Width
			settings.Height = existing.Height
		} else if err == gorm.ErrRecordNotFound {
			settings.Width = 800
			settings.Height = 480
		} else {
			c.JSON(http.StatusInternalServerError, errorResponse("Unable to load device settings"))
			return
		}
		if settings.ImgUpdateInterval < 1 || settings.DitherStrength < 0 {
			c.JSON(http.StatusBadRequest, errorResponse("Settings values must be positive (dither strength may be zero)"))
			return
		}
		if err := db.Where(DeviceSetting{DeviceID: device.DeviceID}).Assign(&settings).FirstOrCreate(&settings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("Unable to save settings"))
			return
		}
	}
	c.JSON(http.StatusOK, successResponse(map[string]string{"message": "Device updated"}))
}

func handleAdminRevokeDevice(c *gin.Context, db *gorm.DB) {
	var device Device
	if err := db.Where("device_id = ?", c.Param("deviceID")).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, errorResponse("Device not found"))
		return
	}
	device.DeviceToken = ""
	device.Enabled = false
	if err := db.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Unable to revoke device token"))
		return
	}
	c.JSON(http.StatusOK, successResponse(map[string]string{"message": "Device authorization revoked"}))
}

func handleAdminDeleteDevice(c *gin.Context, db *gorm.DB) {
	deviceID := c.Param("deviceID")
	var device Device
	if err := db.Where("device_id = ?", deviceID).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, errorResponse("Device not found"))
		return
	}

	// Keep the device's related state from becoming orphaned. Generated files are
	// intentionally left in the cache; they are no longer addressable after the
	// assignment records are removed and can be cleaned up by cache maintenance.
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", deviceID).Delete(&DeviceImage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", deviceID).Delete(&DeviceSetting{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", deviceID).Delete(&DeviceTelemetry{}).Error; err != nil {
			return err
		}
		return tx.Delete(&device).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Unable to delete device"))
		return
	}
	c.JSON(http.StatusOK, successResponse(map[string]string{"message": "Device deleted"}))
}

func handleAdminEnableDevice(c *gin.Context, db *gorm.DB) {
	var device Device
	if err := db.Where("device_id = ?", c.Param("deviceID")).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, errorResponse("Device not found"))
		return
	}
	device.Enabled = true
	if err := db.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Unable to enable device enrollment"))
		return
	}
	c.JSON(http.StatusOK, successResponse(map[string]string{"message": "Device enrollment enabled"}))
}

func handleAdminResetDeviceImage(c *gin.Context, db *gorm.DB) {
	var device Device
	if err := db.Where("device_id = ?", c.Param("deviceID")).First(&device).Error; err != nil {
		c.JSON(http.StatusNotFound, errorResponse("Device not found"))
		return
	}
	if err := db.Where("device_id = ?", device.DeviceID).Delete(&DeviceImage{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Unable to clear device image assignments"))
		return
	}
	device.CurrentImage = ""
	device.StartIndex = randomStartIndex(db)
	if err := db.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Unable to reset device image"))
		return
	}
	c.JSON(http.StatusOK, successResponse(map[string]interface{}{"message": "Image assignment reset", "start_index": device.StartIndex}))
}
