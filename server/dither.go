package main

import (
	"image"
	"image/color"
	"log"

	"github.com/makeworld-the-better-one/dither/v2"
)

const (
	ditherProcessingVersion = 2

	// Ignore a small number of extreme pixels so that specular highlights and
	// deep shadows do not prevent a useful adjustment.
	autoLevelsPercentile = 1
	maxAutoContrast      = 1.35
	maxAutoBrightness    = 20.0
)

var palettes = map[string][]color.Color{
	"7Standard": {
		color.RGBA{0, 0, 0, 255},       // Black
		color.RGBA{255, 255, 255, 255}, // White
		color.RGBA{0, 0, 255, 255},     // Blue
		color.RGBA{0, 255, 0, 255},     // Green
		color.RGBA{255, 0, 0, 255},     // Red
		color.RGBA{255, 255, 0, 255},   // Yellow
		color.RGBA{255, 165, 0, 255},   // Orange
	},
	"7Eink": {
		color.RGBA{49, 40, 56, 255},    // lab(17.6, 8.3, -8.9) Dark state (DS)
		color.RGBA{174, 173, 168, 255}, // lab(70.6, -0.4, 2.4) White state (WS)
		color.RGBA{57, 63, 104, 255},   // lab(28, 9.2, -25) Blue state (BS)
		color.RGBA{48, 101, 68, 255},   // lab(38.3, -26, 13.4) Green state (GS)
		color.RGBA{146, 61, 62, 255},   // lab(37.6, 35.9, 17.4) Red state (RS)
		color.RGBA{173, 160, 73, 255},  // lab(65.5, -6.7, 46.4) Yellow state (YS)
		color.RGBA{160, 83, 65, 255},   // lab(44.4, 30, 24.9) Orange state (OS)
	},
}

// define available dither algorithms
// https://pkg.go.dev/github.com/makeworld-the-better-one/dither/v2
var error_dither_algo = map[string]dither.ErrorDiffusionMatrix{
	"Atkinson":            dither.Atkinson,
	"Burkes":              dither.Burkes,
	"FalseFloydSteinberg": dither.FalseFloydSteinberg,
	"FloydSteinberg":      dither.FloydSteinberg,
	"JarvisJudiceNinke":   dither.JarvisJudiceNinke,
	"Sierra":              dither.Sierra,
	"Sierra2":             dither.Sierra2,
	"Sierra2_4A":          dither.Sierra2_4A,
	"Sierra3":             dither.Sierra3,
	"SierraLite":          dither.SierraLite,
	"Simple2D":            dither.Simple2D,
	"StevenPigeon":        dither.StevenPigeon,
	"Stucki":              dither.Stucki,
	"TwoRowSierra":        dither.TwoRowSierra}

var ordered_dither_algo = map[string]dither.OrderedDitherMatrix{
	"ClusteredDot4x4":            dither.ClusteredDot4x4,
	"ClusteredDot6x6":            dither.ClusteredDot6x6,
	"ClusteredDot6x6_2":          dither.ClusteredDot6x6_2,
	"ClusteredDot6x6_3":          dither.ClusteredDot6x6_3,
	"ClusteredDot8x8":            dither.ClusteredDot8x8,
	"ClusteredDotDiagonal16x16":  dither.ClusteredDotDiagonal16x16,
	"ClusteredDotDiagonal6x6":    dither.ClusteredDotDiagonal6x6,
	"ClusteredDotDiagonal8x8":    dither.ClusteredDotDiagonal8x8,
	"ClusteredDotDiagonal8x8_2":  dither.ClusteredDotDiagonal8x8_2,
	"ClusteredDotDiagonal8x8_3":  dither.ClusteredDotDiagonal8x8_3,
	"ClusteredDotHorizontalLine": dither.ClusteredDotHorizontalLine,
	"ClusteredDotSpiral5x5":      dither.ClusteredDotSpiral5x5,
	"ClusteredDotVerticalLine":   dither.ClusteredDotVerticalLine,
	"Horizontal3x5":              dither.Horizontal3x5,
	"Vertical5x3":                dither.Vertical5x3,
}

func fetchAndDither(file string, selectedPalette string, selectedDitherAlgorithm string, ditherStrength float32, autoBrightness bool, autoContrast bool, targetWidth int, targetHeight int, resizeMethod string) image.Image {

	// Define default options
	if selectedPalette == "" {
		selectedPalette = "7Standard"
	}
	if selectedDitherAlgorithm == "" {
		selectedDitherAlgorithm = "StevenPigeon"
	}

	strength := float32(ditherStrength)

	d := dither.NewDitherer(palettes[selectedPalette])
	d.Serpentine = true

	if _, ok := error_dither_algo[selectedDitherAlgorithm]; ok {
		d.Matrix = dither.ErrorDiffusionStrength(error_dither_algo[selectedDitherAlgorithm], strength)
	} else if _, ok := ordered_dither_algo[selectedDitherAlgorithm]; ok {
		d.Mapper = dither.PixelMapperFromMatrix(ordered_dither_algo[selectedDitherAlgorithm], strength)
	}

	log.Println("Processing file:", file)
	img, err := loadImage(file)
	if err != nil {
		log.Println("Error loading image:", err)
		return nil
	}
	//resize the image to 800x480
	img = resizeImage(img, targetWidth, targetHeight, "Lanczos", resizeMethod)
	img = autoLevels(img, autoBrightness, autoContrast)

	img = d.Dither(img)
	return img
}

// autoLevels gently expands the luminance range before dithering. E-ink
// palettes have relatively few tones, so this preserves detail in otherwise
// flat or slightly underexposed photos without aggressively clipping them.
// The same adjustment is applied to every RGB channel to retain color balance.
func autoLevels(img image.Image, autoBrightness bool, autoContrast bool) image.Image {
	if !autoBrightness && !autoContrast {
		return img
	}
	bounds := img.Bounds()
	pixelCount := bounds.Dx() * bounds.Dy()
	if pixelCount == 0 {
		return img
	}

	var histogram [256]int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			luminance := (54*int(r>>8) + 183*int(g>>8) + 19*int(b>>8)) >> 8
			histogram[luminance]++
		}
	}

	clipCount := pixelCount * autoLevelsPercentile / 100
	low := percentileFromHistogram(histogram, clipCount)
	high := percentileFromHistogram(histogram, pixelCount-clipCount-1)
	if high <= low {
		return img
	}

	contrast := 1.0
	if autoContrast {
		contrast = minFloat(maxAutoContrast, 220.0/float64(high-low))
	}
	if contrast < 1 {
		contrast = 1
	}
	brightness := 0.0
	if autoBrightness {
		brightness = 127.5 - contrast*float64(low+high)/2
		if brightness > maxAutoBrightness {
			brightness = maxAutoBrightness
		} else if brightness < -maxAutoBrightness {
			brightness = -maxAutoBrightness
		}
	}
	if contrast == 1 && brightness == 0 {
		return img
	}

	adjusted := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			adjusted.SetNRGBA(x, y, color.NRGBA{
				R: adjustLevel(uint8(r>>8), contrast, brightness),
				G: adjustLevel(uint8(g>>8), contrast, brightness),
				B: adjustLevel(uint8(b>>8), contrast, brightness),
				A: uint8(a >> 8),
			})
		}
	}
	return adjusted
}

func percentileFromHistogram(histogram [256]int, target int) int {
	seen := 0
	for value, count := range histogram {
		seen += count
		if seen > target {
			return value
		}
	}
	return 255
}

func adjustLevel(value uint8, contrast, brightness float64) uint8 {
	adjusted := contrast*float64(value) + brightness
	if adjusted <= 0 {
		return 0
	}
	if adjusted >= 255 {
		return 255
	}
	return uint8(adjusted + 0.5)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func imgToBitmap(img image.Image, selectedPalette string, targetWidth int, targetHeight int) [][]bool {
	// Separate the dithered image to bitmap of color channels

	// Create a slice of bitmaps for each color in the palette
	bitmaps := make([][]bool, len(palettes[selectedPalette]))
	for i := range bitmaps {
		bitmaps[i] = make([]bool, targetWidth*targetHeight)
	}
	for i, color := range palettes[selectedPalette] {
		for y := 0; y < targetHeight; y++ {
			for x := 0; x < targetWidth; x++ {
				if img.At(x, y) == color {
					bitmaps[i][int(y*targetWidth+x)] = true
				} else {
					bitmaps[i][int(y*targetWidth+x)] = false
				}
			}
		}
	}
	return bitmaps
}
