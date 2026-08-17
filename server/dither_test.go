package main

import (
	"image"
	"image/color"
	"testing"
)

func TestAutoLevelsExpandsLowContrastImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 1))
	for x := 0; x < 100; x++ {
		value := uint8(90 + x/5) // range 90..109
		img.SetNRGBA(x, 0, color.NRGBA{R: value, G: value, B: value, A: 255})
	}

	adjusted := autoLevels(img, true, true)
	low, _, _, _ := adjusted.At(0, 0).RGBA()
	high, _, _, _ := adjusted.At(99, 0).RGBA()
	if high-low <= 19<<8 {
		t.Fatalf("autoLevels did not expand contrast: got range %d", high-low)
	}
}

func TestAutoLevelsKeepsChannelBalance(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 50, G: 100, B: 150, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 100, G: 150, B: 200, A: 255})

	adjusted := autoLevels(img, true, true)
	r, g, b, _ := adjusted.At(0, 0).RGBA()
	r8, g8, b8 := int(r>>8), int(g>>8), int(b>>8)
	if difference := (g8 - r8) - (b8 - g8); difference < -1 || difference > 1 {
		t.Fatalf("autoLevels changed channel balance: R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}
}
