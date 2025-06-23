package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
    _ "image/jpeg"
	"os"

	"github.com/makeworld-the-better-one/dither/v2"
)

// resizeImage resizes the given image to the specified width and height.
func resizeImage(img image.Image, width, height int, mode ...string) image.Image {
    resizeMode := "fit" // Default mode
    if len(mode) > 0 {
        resizeMode = mode[0]
    }

    srcWidth := img.Bounds().Dx()
    srcHeight := img.Bounds().Dy()
    srcAspect := float64(srcWidth) / float64(srcHeight)
    dstAspect := float64(width) / float64(height)

    var newImg *image.RGBA
    var scaleX, scaleY float64
    var offsetX, offsetY int

    switch resizeMode {
    case "fill": // Fill the target dimensions, may change aspect ratio
        newImg = image.NewRGBA(image.Rect(0, 0, width, height))
        scaleX = float64(srcWidth) / float64(width)
        scaleY = float64(srcHeight) / float64(height)

    case "fit": // Maintain aspect ratio, fit within target dimensions
        if srcAspect > dstAspect {
            // Source is wider, fit to width
            newHeight := int(float64(width) / srcAspect)
            newImg = image.NewRGBA(image.Rect(0, 0, width, newHeight))
            scaleX = float64(srcWidth) / float64(width)
            scaleY = float64(srcHeight) / float64(newHeight)
            height = newHeight
        } else {
            // Source is taller, fit to height
            newWidth := int(float64(height) * srcAspect)
            newImg = image.NewRGBA(image.Rect(0, 0, newWidth, height))
            scaleX = float64(srcWidth) / float64(newWidth)
            scaleY = float64(srcHeight) / float64(height)
            width = newWidth
        }

    case "cut": // Maintain aspect ratio, cut/crop to target dimensions
        newImg = image.NewRGBA(image.Rect(0, 0, width, height))
        if srcAspect > dstAspect {
            // Source is wider, cut sides
            scaleY = float64(srcHeight) / float64(height)
            scaleX = scaleY
            // Center horizontally
            offsetX = int((float64(srcWidth) - float64(width)*scaleX) / 2)
        } else {
            // Source is taller, cut top/bottom
            scaleX = float64(srcWidth) / float64(width)
            scaleY = scaleX
            // Center vertically
            offsetY = int((float64(srcHeight) - float64(height)*scaleY) / 2)
        }

    default: // "stretch" or any other value - stretch to fill target dimensions
        newImg = image.NewRGBA(image.Rect(0, 0, width, height))
        scaleX = float64(srcWidth) / float64(width)
        scaleY = float64(srcHeight) / float64(height)
    }

    // Loop through each pixel in the new image
    for y := 0; y < height; y++ {
        for x := 0; x < width; x++ {
            // Calculate the corresponding pixel in the original image
            origX := int(float64(x) * scaleX) + offsetX
            origY := int(float64(y) * scaleY) + offsetY

            // Get the color from the original image
            c := img.At(origX, origY)

            // Set the color in the new image
            newImg.Set(x, y, c)
        }
    }

    return newImg
}

func main(){
// Replace with your actual image path
imagePath := "../asset/example.jpg"

// Open the image file
file, err := os.Open(imagePath)
if err != nil {
    fmt.Println("Error opening image:", err)
    return
}
defer file.Close()

// Decode the image
img, _, err := image.Decode(file)
if err != nil {
    fmt.Println("Error decoding image:", err)
    return
}

//resize the image to 800x480
img = resizeImage(img, 800, 480)

// These are the colors we want in our output image
// palette_1 := []color.Color{
//     color.RGBA{0, 0, 0, 255},   // Black
//     color.RGBA{255, 255, 255, 255}, // White
//     color.RGBA{0, 0, 255, 255}, // Blue
//     color.RGBA{0, 255, 0, 255}, // Green
//     color.RGBA{255, 0, 0, 255}, // Red
//     color.RGBA{255, 255, 0, 255}, // Yellow
//     color.RGBA{255, 165, 0, 255}, // Orange
// }
palette_2 := []color.Color{
    color.RGBA{49, 40, 56, 255},   // Dark state (DS)
    color.RGBA{174, 173, 168, 255}, // White state (WS)
    color.RGBA{57, 63, 104, 255},   // Blue state (BS)
    color.RGBA{48, 101, 68, 255},   // Green state (GS)
    color.RGBA{146, 61, 62, 255},   // Red state (RS)
    color.RGBA{173, 160, 73, 255},   // Yellow state (YS)
    color.RGBA{160, 83, 65, 255},   // Orange state (OS)
}

// Create ditherer
d := dither.NewDitherer(palette_2)
d.Matrix = dither.Stucki

// Dither the image, attempting to modify the existing image
// If it can't then a dithered copy will be returned.
img = d.Dither(img)

// Save the dithered image to a new file
outputPath := "./dithered_image.png"
outFile, err := os.Create(outputPath)
if err != nil {
    fmt.Println("Error creating output file:", err)
    return
}
defer outFile.Close()
// Encode the dithered image to PNG format
if err := png.Encode(outFile, img); err != nil {
    fmt.Println("Error encoding dithered image:", err)
    return
}}