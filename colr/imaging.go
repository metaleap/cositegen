package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	g "github.com/AllenDang/giu"
	"golang.org/x/image/draw"
)

var (
	imgSrc         [10]*image.RGBA
	imgDst         *image.RGBA
	imgSrcTexture  [10]*g.Texture
	imgDstTexture  *g.Texture
	imgSrcFilePath string
	imgDstFilePath string
	imgSize        image.Rectangle
	imgScaleDown   draw.Interpolator = draw.BiLinear // dont change it!
	imgScaleUp     draw.Interpolator = draw.CatmullRom
)

func imgDstNew(size image.Rectangle) (ret *image.RGBA) {
	ret = image.NewRGBA(image.Rect(0, 0, size.Dx(), size.Dy()))
	for x := 0; x < size.Dx(); x++ {
		for y := 0; y < size.Dy(); y++ {
			ret.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	return
}

func imgSave(img image.Image, filePath string) {
	pngfile, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}
	if err = png.Encode(pngfile, img); err != nil {
		panic(err)
	}
	if err = pngfile.Sync(); err != nil {
		panic(err)
	}
	if err = pngfile.Close(); err != nil {
		panic(err)
	}
}

func imgDstSave() {
	imgSave(imgDst, imgDstFilePath)
}

func imgDstBrush() {
	width := float64(imgSize.Dx()) / float64(brushSize)
	factor, col := (float64(imgSize.Dx()) / width), allColors[idxColSelCur]
	img_small := image.NewRGBA(image.Rect(0, 0, int(width), int(float64(imgSize.Dy())/factor)))
	for _, move := range brushRecording.moves {
		at := image.Pt(int(float64(move.X)/factor), int(float64(move.Y)/factor))
		img_small.SetRGBA(at.X, at.Y, col)
	}
	img_full := image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
	if false {
		imgScaleUp.Scale(img_full, img_full.Bounds(), img_small, img_small.Bounds(), draw.Src, nil)
	} else {
		for _, move := range brushRecording.moves {
			img_full.Set(move.X, move.Y, col)
			fac := 255.0 / (float64(brushSize) + 0.5)
			for x := -brushSize; x <= brushSize; x++ {
				for y := -brushSize; y <= brushSize; y++ {
					a := fac*math.Abs(float64(x)) + math.Abs(float64(y))
					alpha := color.RGBA{col.R, col.G, col.B, uint8(math.Abs(255.0 - a))}
					img_full.Set(move.X+x, move.Y+y, alpha)
				}
			}
		}
	}

	imgSave(img_full, "/dev/shm/tmp.png")
}

func imgSrcEnsurePanelBorders() {
	factor := float64(pageLayout.Page.Dx()) / float64(imgSrc[0].Rect.Dx())
	pageLayout.panels = make([]image.Rectangle, len(pageLayout.Panels))
	for i, panelrect := range pageLayout.Panels {
		pageLayout.panels[i] = image.Rect(
			int(float64(panelrect.Min.X)/factor),
			int(float64(panelrect.Min.Y)/factor),
			int(float64(panelrect.Max.X)/factor),
			int(float64(panelrect.Max.Y)/factor),
		)
	}
	for x := 0; x < imgSize.Dx(); x++ {
		for y := 0; y < imgSize.Dy(); y++ {
			is_pixel_in_any_panel := false
			for _, panelrect := range pageLayout.panels {
				if x >= panelrect.Min.X && x <= panelrect.Max.X &&
					y >= panelrect.Min.Y && y <= panelrect.Max.Y {
					is_pixel_in_any_panel = true
					break
				}
			}
			if !is_pixel_in_any_panel {
				imgSrc[0].SetRGBA(x, y, color.RGBA{A: 255})
			}
		}
	}
}

func imgDownsized(imgSrc *image.RGBA, maxWidth int) (ret *image.RGBA) {
	origwidth, origheight := imgSrc.Bounds().Max.X, imgSrc.Bounds().Max.Y
	newheight := int(float64(origheight) / (float64(origwidth) / float64(maxWidth)))
	ret = image.NewRGBA(image.Rect(0, 0, maxWidth, newheight))
	imgScaleDown.Scale(ret, ret.Bounds(), imgSrc, imgSrc.Bounds(), draw.Src, nil)
	// greys into blacks for our purposes here:
	for x := 0; x < imgSize.Dx(); x++ {
		for y := 0; y < imgSize.Dy(); y++ {
			r, g, b, a := ret.At(x, y).RGBA()
			if !(r == 0 && g == 0 && b == 0) {
				panic("stuff has changed, update code!")
			}
			if a != 0 {
				ret.Set(x, y, color.RGBA{A: 255})
			}
		}
	}
	return
}
