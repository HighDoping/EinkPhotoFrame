package main

import (
	"fmt"
	"time"

	"github.com/HighDoping/EinkPhotoFrame/config"
	"gorm.io/gorm"
)

type imageUpdateResult struct {
	NoUpdate  bool
	ImageUUID string
	Files     []string
}

func processDeviceImageUpdate(db *gorm.DB, cfg config.Config, device *Device, force bool) (imageUpdateResult, error) {
	var settings DeviceSetting
	result := db.Where(&DeviceSetting{DeviceID: device.DeviceID}).First(&settings)
	if result.Error != nil {
		return imageUpdateResult{}, result.Error
	}

	if !force && device.UpdatedAt.Add(time.Duration(settings.ImgUpdateInterval)*time.Second).After(time.Now()) && device.CurrentImage != "" {
		return imageUpdateResult{NoUpdate: true}, nil
	}

	nextImage, err := getNextRandom(db, *device)
	if err != nil {
		return imageUpdateResult{}, err
	}

	ditheredImage, err := getDithered(db, nextImage, settings.Palette, settings.DitherAlgorithm, settings.DitherStrength, settings.AutoBrightness, settings.AutoContrast, settings.Width, settings.Height, settings.ResizeMethod, cfg)
	if err != nil {
		return imageUpdateResult{}, err
	}

	ditheredImg, err := loadImage(ditheredImage.Path)
	if err != nil {
		return imageUpdateResult{}, err
	}

	ditheredImgBit := imgToBitmap(ditheredImg, settings.Palette, settings.Width, settings.Height)
	print(len(ditheredImgBit))
	filepaths := make([]string, len(ditheredImgBit))
	for i := 0; i < len(ditheredImgBit); i++ {
		bytesData := BitsToBytes(ditheredImgBit[i])
		assetUUID := generateUUID()
		filePath := fmt.Sprintf("%s/%s.bin", cfg.CacheDir, assetUUID)
		err = saveBytesToFile(filePath, bytesData)
		if err != nil {
			return imageUpdateResult{}, err
		}
		assignment := DeviceImage{AssetUUID: assetUUID, DeviceID: device.DeviceID, ImageUUID: nextImage.UUID, FileIndex: i, Path: filePath}
		if err := db.Create(&assignment).Error; err != nil {
			return imageUpdateResult{}, err
		}
		filepaths[i] = "assets/" + assetUUID
	}

	device.CurrentImage = nextImage.UUID
	device.UpdatedAt = time.Now()
	db.Save(device)

	return imageUpdateResult{
		NoUpdate:  false,
		ImageUUID: nextImage.UUID,
		Files:     filepaths,
	}, nil
}
