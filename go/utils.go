package main

import (
	"image"

	"log"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
)

func loadImage(path string) (image.Image, error) {
	img, err := imgio.Open(path)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return img, nil
}

func saveImage(img image.Image, path string) error {
	// Save the image to the specified path using PNG format
	encoder := imgio.PNGEncoder()
	if err := imgio.Save(path, img, encoder); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func resizeImage(img image.Image, width, height int, filter string, method string) image.Image {

	resize_algo := map[string]transform.ResampleFilter{
		"Linear":            transform.Linear,
		"NearestNeighbor":   transform.NearestNeighbor,
		"Box":               transform.Box,
		"Gaussian":          transform.Gaussian,
		"MitchellNetravali": transform.MitchellNetravali,
		"CatmullRom":        transform.CatmullRom,
		"Lanczos":           transform.Lanczos,
	}

	// Default to NearestNeighbor if filter is empty or not in map
	resampleFilter, ok := resize_algo[filter]
	if !ok || filter == "" {
		resampleFilter = transform.NearestNeighbor
	}

	// method: cut or fill to fit the image
	imgWidth := img.Bounds().Dx()
	imgHeight := img.Bounds().Dy()
	aspectRatio := float64(imgWidth) / float64(imgHeight)
	targetAspectRatio := float64(width) / float64(height)

	if method == "cut" {
		// Crop the image to match target aspect ratio
		if aspectRatio > targetAspectRatio {
			// Image is wider, crop width
			newWidth := int(float64(imgHeight) * targetAspectRatio)
			xOffset := (imgWidth - newWidth) / 2
			img = transform.Crop(img, image.Rect(xOffset, 0, xOffset+newWidth, imgHeight))
		} else if aspectRatio < targetAspectRatio {
			// Image is taller, crop height
			newHeight := int(float64(imgWidth) / targetAspectRatio)
			yOffset := (imgHeight - newHeight) / 2
			img = transform.Crop(img, image.Rect(0, yOffset, imgWidth, yOffset+newHeight))
		}
	} else if method == "fill_white" || method == "fill_black" {
		// Fill with white or black
		fillColor := image.White
		if method == "fill_black" {
			fillColor = image.Black
		}

		var canvas *image.RGBA
		var xOffset, yOffset int

		if aspectRatio > targetAspectRatio {
			// Image is wider, adjust height
			newHeight := int(float64(imgWidth) / targetAspectRatio)
			canvas = image.NewRGBA(image.Rect(0, 0, imgWidth, newHeight))
			yOffset = (newHeight - imgHeight) / 2
		} else {
			// Image is taller, adjust width
			newWidth := int(float64(imgHeight) * targetAspectRatio)
			canvas = image.NewRGBA(image.Rect(0, 0, newWidth, imgHeight))
			xOffset = (newWidth - imgWidth) / 2
		}

		// Fill canvas with background color
		bounds := canvas.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				canvas.Set(x, y, fillColor)
			}
		}

		// Draw the original image centered on canvas
		for y := 0; y < imgHeight; y++ {
			for x := 0; x < imgWidth; x++ {
				canvas.Set(x+xOffset, y+yOffset, img.At(x, y))
			}
		}
		img = canvas
	}
	// Resize the image to the specified width and height
	resizedImg := transform.Resize(img, width, height, resampleFilter)
	return resizedImg
}
