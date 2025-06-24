package main

import (
	"fmt"
	"image/color"
	"log"
    "os"
    "path/filepath"
    "strings"

	"github.com/makeworld-the-better-one/dither/v2"
)

func generateFileList(folder string,file_ext []string) ([]string, error) {
    files, err := os.ReadDir(folder)
    if err != nil {
        return nil, err
    }
    var fileList []string
    for _, file := range files {
        if !file.IsDir() {
            for _, ext := range file_ext {
                if strings.HasSuffix(file.Name(), ext) {
                    fileList = append(fileList, filepath.Join(folder, file.Name()))
                    break
                }
            }
        }
    }
    return fileList, nil
}

func main() {

    // Define available palettes
palettes := map[string][]color.Color{
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
for palette,_ := range palettes{
    selectedPalette := palette

for k, _ := range error_dither_algo {
    selectedDitherAlgorithm := k
// selectedPalette := "7Standard"
// selectedDitherAlgorithm := "FloydSteinberg"
strength := float32(1.0)

d := dither.NewDitherer(palettes[selectedPalette])
d.Serpentine = true

if _, ok := error_dither_algo[selectedDitherAlgorithm]; ok {
    d.Matrix = dither.ErrorDiffusionStrength(error_dither_algo[selectedDitherAlgorithm],strength)
} else if _, ok := ordered_dither_algo[selectedDitherAlgorithm]; ok {
    d.Mapper = dither.PixelMapperFromMatrix(ordered_dither_algo[selectedDitherAlgorithm],strength)
}


folder:="../asset"

//generate a list of files in folder
file_ext := []string{".jpg", ".jpeg", ".png", ".bmp", ".gif"}
fileList, err := generateFileList(folder, file_ext)
if err != nil {
    log.Println("Error generating file list:", err)
    return
}


// Load the image from list
for _, file := range fileList {
    log.Println("Processing file:", file)
    file_name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
    img, err := loadImage(file)
    if err != nil {
        log.Println("Error loading image:", err)
        continue
    }
    //resize the image to 800x480
    img = resizeImage(img, 800, 480,"Lanczos", "cut")

    img = d.Dither(img)

    // Save the dithered image to a new file
    outputPath := fmt.Sprintf("dithered_%s_%s_%s.png",file_name, selectedPalette, selectedDitherAlgorithm)
    if err := saveImage(img, outputPath); err != nil {
        log.Println("Error saving image:", err)
        return
    }
    log.Println("Dithered image saved to", outputPath)}}}}