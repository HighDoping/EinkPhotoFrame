package main

import (
	"image"
	"image/color"
	"log"

	"github.com/makeworld-the-better-one/dither/v2"
)
var palettes = map[string][]color.Color{
    "7Standard": {
        color.RGBA{0, 0, 0, 255},      // Black
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

//define available dither algorithms
//https://pkg.go.dev/github.com/makeworld-the-better-one/dither/v2
var error_dither_algo = map[string]dither. ErrorDiffusionMatrix{
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

var ordered_dither_algo = map[string]dither.OrderedDitherMatrix{
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

func fetchandDither(file string,selectedPalette string,selectedDitherAlgorithm string,targetWidth int, targetHeight int)image.Image{

    // Define default options
    if selectedPalette == "" {
        selectedPalette = "7Standard"
    }
    if selectedDitherAlgorithm == "" {
        selectedDitherAlgorithm = "StevenPigeon"
    }
    


    strength := float32(1.0)

    d := dither.NewDitherer(palettes[selectedPalette])
    d.Serpentine = true

    if _, ok := error_dither_algo[selectedDitherAlgorithm]; ok {
        d.Matrix = dither.ErrorDiffusionStrength(error_dither_algo[selectedDitherAlgorithm],strength)
    } else if _, ok := ordered_dither_algo[selectedDitherAlgorithm]; ok {
        d.Mapper = dither.PixelMapperFromMatrix(ordered_dither_algo[selectedDitherAlgorithm],strength)
    }

    log.Println("Processing file:", file)
    img, err := loadImage(file)
    if err != nil {
        log.Println("Error loading image:", err)
        return nil
    }
    //resize the image to 800x480
    img = resizeImage(img, targetWidth,targetHeight,"Lanczos", "cut")

    img = d.Dither(img)
    return img
}

func imgToBitmap(img image.Image, selectedPalette string, targetWidth int, targetHeight int) [][]bool{
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
                    bitmaps[i][int(y*targetWidth+x)]= true
                } else {
                    bitmaps[i][int(y*targetWidth+x)]= false
                }
            }
        }
    }
    return bitmaps
}

