package main

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
)

// func imgPanels (bwPng )

func imgToMonochrome(srcImgData io.Reader, blackIfLessThan uint8) []byte {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}

	imggray := image.NewGray(image.Rect(0, 0, imgsrc.Bounds().Max.X, imgsrc.Bounds().Max.Y))
	for px := 0; px < imgsrc.Bounds().Max.X; px++ {
		for py := 0; py < imgsrc.Bounds().Max.Y; py++ {
			var colbw uint8
			// ensure grayscale
			switch colsrc := imgsrc.At(px, py).(type) {
			case color.Gray:
				colbw = colsrc.Y
			case color.RGBA:
				colbw = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			case color.NRGBA:
				colbw = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			default:
				panic(colsrc)
				// panic(fmt.Sprintf("%T", colsrc))
			}

			// now black&white-only
			if colbw < blackIfLessThan {
				colbw = 0
			} else {
				colbw = 255
			}

			imggray.Set(px, py, color.Gray{Y: colbw})
		}
	}

	var pngbuf bytes.Buffer
	if err = png.Encode(&pngbuf, imggray); err != nil {
		panic(err)
	}
	return pngbuf.Bytes()
}
