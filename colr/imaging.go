package main

import (
	"image"
	"image/color"
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
	factor := float64(pageLayout.Page.Dx()) / float64(imgSrc.Rect.Dx())
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
				imgSrc.SetRGBA(x, y, color.RGBA{A: 255})
			}
		}
	}
}
