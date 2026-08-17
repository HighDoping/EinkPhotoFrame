package main

import "time"

// Standard API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Define models
type Device struct {
	ID           uint   `gorm:"primarykey"`
	DeviceID     string `gorm:"uniqueIndex;not null"`
	DeviceName   string `gorm:"not null"`
	DeviceToken  string `gorm:"unique"`
	Enabled      bool   `gorm:"not null;default:true"`
	StartIndex   uint   `gorm:"not null;default:0"`
	CurrentImage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DeviceSetting struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	DeviceID          string    `gorm:"not null" json:"device_id"`
	ImgUpdateInterval int       `gorm:"not null;default:600" json:"img_update_interval"`
	Height            int       `gorm:"not null;default:480" json:"height"`
	Width             int       `gorm:"not null;default:800" json:"width"`
	Rotation          int       `gorm:"not null;default:0" json:"rotation"`
	Palette           string    `gorm:"not null;default:'7Standard'" json:"palette"`
	DitherAlgorithm   string    `gorm:"not null;default:'StevenPigeon'" json:"dither_algorithm"`
	DitherStrength    float32   `gorm:"not null;default:1.0" json:"dither_strength"`
	AutoBrightness    bool      `gorm:"not null;default:true" json:"auto_brightness"`
	AutoContrast      bool      `gorm:"not null;default:true" json:"auto_contrast"`
	ResizeMethod      string    `gorm:"not null;default:'cut'" json:"resize_method"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Device            Device    `gorm:"foreignKey:DeviceID;references:DeviceID" json:"-"`
}

type DeviceTelemetry struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	DeviceID     string    `gorm:"uniqueIndex;not null" json:"device_id"`
	BatteryLevel int       `gorm:"not null;default:100" json:"battery_level"`
	LastSeen     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"last_seen"`
	Device       Device    `gorm:"foreignKey:DeviceID;references:DeviceID" json:"-"`
}

// DeviceImage authorizes one generated asset for exactly one device.
type DeviceImage struct {
	ID        uint   `gorm:"primarykey"`
	AssetUUID string `gorm:"uniqueIndex;not null"`
	DeviceID  string `gorm:"index;not null"`
	ImageUUID string `gorm:"index;not null"`
	FileIndex int    `gorm:"not null"`
	Path      string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	Device    Device `gorm:"foreignKey:DeviceID;references:DeviceID"`
}

type DBImage struct {
	ID             uint   `gorm:"primarykey"`
	Path           string `gorm:"uniqueIndex;not null"`
	UUID           string `gorm:"uniqueIndex;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DitheredImages []DitheredImage `gorm:"foreignKey:DBImageUUID;references:UUID"`
}

type DitheredImage struct {
	ID                uint    `gorm:"primarykey"`
	UUID              string  `gorm:"not null"`
	DBImageUUID       string  `gorm:"not null"` // Foreign key to DBImage
	ProcessingVersion int     `gorm:"not null;default:1"`
	Palette           string  `gorm:"not null"`
	DitherAlgorithm   string  `gorm:"not null"`
	DitherStrength    float32 `gorm:"not null;default:1.0"`
	AutoBrightness    bool    `gorm:"not null;default:true"`
	AutoContrast      bool    `gorm:"not null;default:true"`
	Height            int     `gorm:"not null;default:480"`
	Width             int     `gorm:"not null;default:800"`
	ResizeMethod      string  `gorm:"not null;default:'cut'"`
	Path              string  `gorm:"uniqueIndex;not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type RandomImage struct {
	ID   uint   `gorm:"primarykey"`
	UUID string `gorm:"uniqueIndex;not null"`
}
