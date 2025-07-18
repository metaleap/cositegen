package main

import (
	"image/color"
)

var (
	allColors   []color.RGBA
	keyedColors [][]color.RGBA
	colorLabels [216]string
	idxCurColor int
)

func init() {
	var n = []uint8{0x33, 0x55, 0x88, 0xAA, 0xCC, 0xEE}
	for _, r := range n {
		for _, g := range n {
			for _, b := range n {
				allColors = append(allColors, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	allColors[0] = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	idx_color := 0
	for letter := 1; letter <= 24; letter++ {
		var digits []color.RGBA
		for digit := 1; digit <= 9; digit++ {
			colorLabels[idx_color] = string('A'+(letter-1)) + string('1'+(digit-1))
			digits = append(digits, allColors[idx_color])
			idx_color++
		}
		keyedColors = append(keyedColors, digits)
	}
}
