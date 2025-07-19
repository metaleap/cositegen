package main

import (
	"image"
	"image/color"

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
	imgScaler      draw.Interpolator = draw.BiLinear // dont change it!
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

func imgSrcEnsurePanelBorders() {
	factor := float64(pageLayout.Page.Dx()) / float64(imgSrc[0].Rect.Dx())
	panelrects := make([]image.Rectangle, len(pageLayout.Panels))
	for i, panelrect := range pageLayout.Panels {
		panelrects[i] = image.Rect(
			int(float64(panelrect.Min.X)/factor),
			int(float64(panelrect.Min.Y)/factor),
			int(float64(panelrect.Max.X)/factor),
			int(float64(panelrect.Max.Y)/factor),
		)
	}
	for x := 0; x < imgSize.Dx(); x++ {
		for y := 0; y < imgSize.Dy(); y++ {
			is_pixel_in_any_panel := false
			for _, panelrect := range panelrects {
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
	imgScaler.Scale(ret, ret.Bounds(), imgSrc, imgSrc.Bounds(), draw.Src, nil)
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
