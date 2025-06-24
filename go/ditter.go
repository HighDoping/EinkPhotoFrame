package main

import (
	"fmt"
	"image/color"
	"log"

	"github.com/makeworld-the-better-one/dither/v2"
)


func main() {

// Load the image from a file
img, err := loadImage("../asset/example.jpg")
if err != nil {
    log.Println("Error loading image:", err)
    return
}

//resize the image to 800x480
img = resizeImage(img, 800, 480,"Lanczos", "fill_black")

// Define available palettes
palettes := map[string][]color.Color{
    "standard": {
        color.RGBA{0, 0, 0, 255},      // Black
        color.RGBA{255, 255, 255, 255}, // White
        color.RGBA{0, 0, 255, 255},     // Blue
        color.RGBA{0, 255, 0, 255},     // Green
        color.RGBA{255, 0, 0, 255},     // Red
        color.RGBA{255, 255, 0, 255},   // Yellow
        color.RGBA{255, 165, 0, 255},   // Orange
    },
    "eink": {
        color.RGBA{49, 40, 56, 255},    // Dark state (DS)
        color.RGBA{174, 173, 168, 255}, // White state (WS)
        color.RGBA{57, 63, 104, 255},   // Blue state (BS)
        color.RGBA{48, 101, 68, 255},   // Green state (GS)
        color.RGBA{146, 61, 62, 255},   // Red state (RS)
        color.RGBA{173, 160, 73, 255},  // Yellow state (YS)
        color.RGBA{160, 83, 65, 255},   // Orange state (OS)
    },
}

//define available dither algorithms
//https://pkg.go.dev/github.com/makeworld-the-better-one/dither/v2
error_dither_algo := map[string]dither. ErrorDiffusionMatrix{
    "Atkinson": dither.Atkinson,
    "Burkes": dither.Burkes,    
    "FalseFloydSteinberg": dither.FalseFloydSteinberg,
    "FloydSteinberg": dither.FloydSteinberg,    
    "JarvisJudiceNinke": dither.JarvisJudiceNinke,
    "Sierra": dither.Sierra,
    "Sierra2": dither.Sierra2,
    "Sierra2_4A": dither.Sierra2_4A,
    "Sierra3": dither.Sierra3,
    "SierraLite": dither.SierraLite,
    "Simple2D": dither.Simple2D,
    "StevenPigeon": dither.StevenPigeon,
    "Stucki": dither.Stucki,
    "TwoRowSierra": dither.TwoRowSierra}

ordered_dither_algo := map[string]dither.OrderedDitherMatrix{
    "ClusteredDot4x4": dither.ClusteredDot4x4,
    "ClusteredDot6x6": dither.ClusteredDot6x6,
    "ClusteredDot6x6_2": dither.ClusteredDot6x6_2,
    "ClusteredDot6x6_3": dither.ClusteredDot6x6_3,
    "ClusteredDot8x8": dither.ClusteredDot8x8,
    "ClusteredDotDiagonal16x16": dither.ClusteredDotDiagonal16x16,
    "ClusteredDotDiagonal6x6": dither.ClusteredDotDiagonal6x6,
    "ClusteredDotDiagonal8x8": dither.ClusteredDotDiagonal8x8,
    "ClusteredDotDiagonal8x8_2": dither.ClusteredDotDiagonal8x8_2,
    "ClusteredDotDiagonal8x8_3": dither.ClusteredDotDiagonal8x8_3,
    "ClusteredDotHorizontalLine": dither.ClusteredDotHorizontalLine,
    "ClusteredDotSpiral5x5": dither.ClusteredDotSpiral5x5,
    "ClusteredDotVerticalLine": dither.ClusteredDotVerticalLine,
    "Horizontal3x5": dither.Horizontal3x5,
    "Vertical5x3": dither.Vertical5x3,
}

selectedPalette := "standard"
selectedDitherAlgorithm := "FloydSteinberg"

strength := float32(1.0)

d := dither.NewDitherer(palettes[selectedPalette])
d.Serpentine = true

if _, ok := error_dither_algo[selectedDitherAlgorithm]; ok {
    d.Matrix = dither.ErrorDiffusionStrength(error_dither_algo[selectedDitherAlgorithm],strength)
} else if _, ok := ordered_dither_algo[selectedDitherAlgorithm]; ok {
    d.Mapper = dither.PixelMapperFromMatrix(ordered_dither_algo[selectedDitherAlgorithm],strength)
}

img = d.Dither(img)

// Save the dithered image to a new file
outputPath := fmt.Sprintf("dithered_%s_%s.png", selectedPalette, selectedDitherAlgorithm)
if err := saveImage(img, outputPath); err != nil {
    log.Println("Error saving image:", err)
    return
}

log.Println("Dithered image saved to", outputPath)}