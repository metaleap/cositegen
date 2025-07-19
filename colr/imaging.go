package main

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/anthonynsimon/bild/blur"
	"golang.org/x/image/draw"
)

var (
	imgSrc         [10]*image.RGBA
	imgDst         *image.RGBA
	imgDstOrig     *image.RGBA
	imgDstPreview  *image.RGBA
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
	imgDstOrig = imgDst
}

func imgDstReload() {
	brushRecording.is, brushRecording.moves, brushRecording.idxPanel = false, nil, -1
	guiUpdateTex(&imgDstPreviewTex, nil)
	imgDstPreview = nil
	redoStack, undoStack = nil, append(undoStack, imgDst)

	imgDst = imgDstOrig
	guiUpdateTex(&imgDstTex, imgDst)
}

func imgDstBrushHaltRec(apply bool) {
	brushRecording.is = false
	if apply {
		imgDstBrush()
	}
}

func imgDstBrush() {
	col, div := allColors[idxColSelCur], 2 // 2 means 1/4 orig size, better not go smaller, as curves become ever more angular turns
	img_small := image.NewRGBA(image.Rect(0, 0, imgSize.Dx()/div, imgSize.Dy()/div))
	moves := make([]image.Point, len(brushRecording.moves))
	for i, move := range brushRecording.moves {
		moves[i] = image.Pt(move.X/div, move.Y/div)
	}
	// first: connect the dots
	for i := 1; i < len(moves); i++ {
		cur, prev := moves[i], moves[i-1]
		xdiff, ydiff := Max(cur.X, prev.X)-Min(cur.X, prev.X), Max(cur.Y, prev.Y)-Min(cur.Y, prev.Y)
		if (xdiff > 1) || (ydiff > 1) {
			move := image.Pt(
				Min(cur.X, prev.X)+(Max(cur.X, prev.X)-Min(cur.X, prev.X))/2,
				Min(cur.Y, prev.Y)+(Max(cur.Y, prev.Y)-Min(cur.Y, prev.Y))/2,
			)
			if !(move.Eq(prev) || move.Eq(cur)) {
				moves = append(moves[:i], append([]image.Point{move}, moves[i:]...)...)
				i--
			}
		}
	}
	// second: draw the dots and fill a circle around it
	for _, move := range moves {
		// draw the point
		img_small.Set(move.X, move.Y, col)
		// brush's circle around it
		imgDrawCircle(img_small, &col, move, brushSize/div, true)
	}
	img_small = blur.Box(img_small, float64(brushSize/div))
	img_full := image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
	imgScaleUp.Scale(img_full, img_full.Bounds(), img_small, img_small.Bounds(), draw.Src, nil)
	img_full = blur.Box(img_full, float64(brushSize/div/2))
	imgDstPreview = image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
	draw.Copy(imgDstPreview, image.Pt(0, 0), imgDst, imgDst.Bounds(), draw.Src, nil)
	draw.Copy(imgDstPreview, image.Pt(0, 0), img_full, img_full.Bounds(), draw.Over, nil)
	guiUpdateTex(&imgDstPreviewTex, imgDstPreview)
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

// https://schwarzers.com/algorithms/ | https://en.wikipedia.org/wiki/Midpoint_circle_algorithm#Jesko's_Method
func imgDrawCircle(img *image.RGBA, col *color.RGBA, pos image.Point, r int, filled bool) {
	mx, my, t1, t2, x, y, pts := pos.X, pos.Y, r/16, 0, r, 0, make([]image.Point, 0, (r*2)*(r*2))
	for x >= y {
		pts = append(pts,
			image.Pt(mx+x, my+y),
			image.Pt(mx+x, my-y),
			image.Pt(mx-x, my+y),
			image.Pt(mx-x, my-y),
			image.Pt(mx+y, my+x),
			image.Pt(mx+y, my-x),
			image.Pt(mx-y, my+x),
			image.Pt(mx-y, my-x))
		y = y + 1
		t1 = t1 + y
		t2 = t1 - x
		if t2 >= 0 {
			t1 = t2
			x = x - 1
		}
	}
	for _, pt := range pts {
		img.Set(pt.X, pt.Y, col)
	}
	if filled && r > 1 {
		imgDrawCircle(img, col, pos, r-1, true)
	}
}
