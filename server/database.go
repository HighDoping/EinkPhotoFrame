package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

func addDithered(db *gorm.DB, image DBImage, palette string, ditherAlgorithm string, ditherStrength float32, targetWidth int, targetHeight int, resizeMethod string) (DitheredImage, error) {
	if db == nil {
		return DitheredImage{}, fmt.Errorf("database connection is nil")
	}
	// Generate path for dithered image
	uuid := generateUUID()
	path := fmt.Sprintf("%s/dithered_%s.png", cacheDir, uuid)
	img := fetchAndDither(image.Path, palette, ditherAlgorithm, ditherStrength, targetWidth, targetHeight, resizeMethod)
	if img == nil {
		return DitheredImage{}, fmt.Errorf("failed to dither image: %s", image.Path)
	}
	// Save dithered image to cache
	err := saveImage(path, img)
	if err != nil {
		return DitheredImage{}, fmt.Errorf("failed to save dithered image to file: %w", err)
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
		return DitheredImage{}, fmt.Errorf("failed to insert dithered image into database: %w", result.Error)
	}

	log.Printf("Inserted dithered image with UUID: %s\n", dithered.UUID)
	return dithered, nil

}

func getDithered(db *gorm.DB, image DBImage, palette string, ditherAlgorithm string, ditherStrength float32, targetWidth int, targetHeight int, resizeMethod string) (DitheredImage, error) {
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
			dithered, err := addDithered(db, image, palette, ditherAlgorithm, ditherStrength, targetWidth, targetHeight, resizeMethod)
			if err != nil {
				return DitheredImage{}, fmt.Errorf("failed to create dithered image: %w", err)
			}
			return dithered, nil
		}

		return DitheredImage{}, fmt.Errorf("failed to query dithered image: %w", result.Error)
	}
	// return found dithered image
	return dithered, nil
}

func removeDithered(db *gorm.DB, uuid string, palette string, ditherAlgorithm string, ditherStrength float32, targetWidth int, targetHeight int, resizeMethod string) error {
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
		result := db.Where(&RandomImage{UUID: device.CurrentImage}).First(&currentRandom)

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

		if err := db.Where(DBImage{UUID: currentRandom.UUID}).First(&nextImage).Error; err != nil {
			return nextImage, fmt.Errorf("failed to find image with UUID: %s", currentRandom.UUID)
		}
	}

	return nextImage, nil
}
