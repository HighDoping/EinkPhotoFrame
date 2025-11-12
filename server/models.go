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
	CurrentImage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DeviceSetting struct {
	ID                uint    `gorm:"primarykey"`
	DeviceID          string  `gorm:"not null"`
	ImgUpdateInterval int     `gorm:"not null;default:600"`
	Height            int     `gorm:"not null;default:480"`
	Width             int     `gorm:"not null;default:800"`
	Rotation          int     `gorm:"not null;default:0"`
	Palette           string  `gorm:"not null;default:'7Standard'"`
	DitherAlgorithm   string  `gorm:"not null;default:'StevenPigeon'"`
	DitherStrength    float32 `gorm:"not null;default:1.0"`
	ResizeMethod      string  `gorm:"not null;default:'cut'"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Device            Device `gorm:"foreignKey:DeviceID;references:DeviceID"`
}

type DeviceTelemetry struct {
	ID           uint      `gorm:"primarykey"`
	DeviceID     string    `gorm:"not null"`
	BatteryLevel int       `gorm:"not null;default:100"`
	LastSeen     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	Device       Device    `gorm:"foreignKey:DeviceID;references:DeviceID"`
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
	ID              uint    `gorm:"primarykey"`
	UUID            string  `gorm:"not null"`
	DBImageUUID     string  `gorm:"not null"` // Foreign key to DBImage
	Palette         string  `gorm:"not null"`
	DitherAlgorithm string  `gorm:"not null"`
	DitherStrength  float32 `gorm:"not null;default:1.0"`
	Height          int     `gorm:"not null;default:480"`
	Width           int     `gorm:"not null;default:800"`
	ResizeMethod    string  `gorm:"not null;default:'cut'"`
	Path            string  `gorm:"uniqueIndex;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type RandomImage struct {
	ID   uint   `gorm:"primarykey"`
	UUID string `gorm:"uniqueIndex;not null"`
}
