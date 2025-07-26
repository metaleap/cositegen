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
	ptZ              image.Point
	imgSrc           [10]*image.RGBA
	imgDst           *image.RGBA
	imgDstOrig       *image.RGBA
	imgDstPreview    *image.RGBA
	imgSrcFilePath   string
	imgDstFilePath   string
	imgSize          image.Rectangle
	imgScaleDown     draw.Interpolator = draw.BiLinear // dont change it!
	imgScaleUp       draw.Interpolator = draw.CatmullRom
	blurModeGaussian bool
	blurSizeFactor   = 0.0
	blurSizeFactors  = []float64{0, 0.11, 0.44, 0.77, 1, 2, 3, 4}
	fillPixelSize    = 5
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
	if imgDst != imgDstOrig {
		imgSave(imgDst, imgDstFilePath)
		imgDstOrig = imgDst
		guiMsg("Saved: %s", imgDstFilePath)
	}
}

func imgDstReload() {
	guiBrush.isRec, guiBrush.moves, guiBrush.idxPanel = false, nil, -1
	guiUpdateTex(&imgDstPreviewTex, nil)
	imgDstPreview = nil
	guiRedoStack, guiUndoStack = nil, append(guiUndoStack, imgDst)

	imgDst = imgDstOrig
	guiUpdateTex(&imgDstTex, imgDst)
	guiMsg("Background colors reloaded (not from disk, but from last save point or the initial load)")
}

func imgDstBrushHaltRec(apply bool) {
	was_rec := guiBrush.isRec
	guiBrush.isRec = false
	if apply && (was_rec || imgDstPreviewTex != nil) {
		imgDstBrushPreview()
	}
}

func imgDstFillPreview() {
	if guiFill.move.Eq(ptZ) { // didnt yet press space
		return
	}
	factor, size := 1.0, imgSrc[idxImgSrc].Bounds()
	if idxImgSrc != 0 {
		for i, idx := 0.95, 1; i >= 0.15; i, idx = i-0.1, idx+1 {
			if idx == idxImgSrc {
				factor = i
				break
			}
		}
	}
	img_small := image.NewRGBA(image.Rect(0, 0, size.Dx(), size.Dy()))
	imgFloodFill(imgSrc[idxImgSrc], img_small, int(factor*float64(guiFill.move.X)), int(factor*float64(guiFill.move.Y)))

	blur_do := If(blurModeGaussian, blur.Gaussian, blur.Box)
	blur_size := blurSizeFactor * (float64(guiBrush.size) * factor)
	if blurSizeFactor > 0.01 {
		img_small = blur_do(img_small, blur_size)
	}
	img_full := img_small
	if idxImgSrc != 0 {
		img_full = image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
		imgScaleUp.Scale(img_full, img_full.Bounds(), img_small, img_small.Bounds(), draw.Src, nil)
		if blurSizeFactor > 0.01 {
			img_full = blur_do(img_full, blur_size*0.5)
		}
	}
	imgDstPreview = image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
	draw.Copy(imgDstPreview, ptZ, imgDst, imgDst.Bounds(), draw.Src, nil)
	draw.Copy(imgDstPreview, ptZ, img_full, img_full.Bounds(), draw.Over, nil)
	guiUpdateTex(&imgDstPreviewTex, imgDstPreview)
}

func imgDstBrushPreview() {
	col, div := allColors[idxColSelCur], 2 // div:=2 means 1/4 orig size, better keep, since with bigger divs brush curves turn ever more angular
	img_small := image.NewRGBA(image.Rect(0, 0, imgSize.Dx()/div, imgSize.Dy()/div))
	moves := make([]image.Point, len(guiBrush.moves))
	for i, move := range guiBrush.moves {
		moves[i] = image.Pt(move.X/div, move.Y/div)
	}
	// first: connect the dots
	for i := 1; i < len(moves); i++ {
		cur, prev := moves[i], moves[i-1]
		xdiff, ydiff := max(cur.X, prev.X)-min(cur.X, prev.X), max(cur.Y, prev.Y)-min(cur.Y, prev.Y)
		if (xdiff > 1) || (ydiff > 1) {
			move := image.Pt(
				min(cur.X, prev.X)+(max(cur.X, prev.X)-min(cur.X, prev.X))/2,
				min(cur.Y, prev.Y)+(max(cur.Y, prev.Y)-min(cur.Y, prev.Y))/2,
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
		imgDrawCircle(img_small, &col, move, guiBrush.size/div, true)
	}
	blur_do := If(blurModeGaussian, blur.Gaussian, blur.Box)
	blur_size := blurSizeFactor * (float64(guiBrush.size) / float64(div))
	if blurSizeFactor > 0.01 {
		img_small = blur_do(img_small, blur_size)
	}
	img_full := image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
	imgScaleUp.Scale(img_full, img_full.Bounds(), img_small, img_small.Bounds(), draw.Src, nil)
	if blurSizeFactor > 0.01 {
		img_full = blur_do(img_full, blur_size*0.5)
	}
	imgDstPreview = image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
	draw.Copy(imgDstPreview, ptZ, imgDst, imgDst.Bounds(), draw.Src, nil)
	draw.Copy(imgDstPreview, ptZ, img_full, img_full.Bounds(), draw.Over, nil)
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
	{ // greys into blacks for our purposes here:
		for x := 0; x < imgSize.Dx(); x++ {
			for y := 0; y < imgSize.Dy(); y++ {
				r, g, b, a := ret.At(x, y).RGBA()
				if !(r == 0 && g == 0 && b == 0) {
					panic("stuff has changed, update code!")
				}
				if a > 1234 {
					ret.Set(x, y, color.RGBA{A: 255})
				}
			}
		}
	}
	return
}

// https://schwarzers.com/algorithms/ | https://en.wikipedia.org/wiki/Midpoint_circle_algorithm#Jesko's_Method
func imgDrawCircle(img *image.RGBA, col *color.RGBA, at image.Point, r int, filled bool) {
	mx, my, t1, t2, x, y, pts := at.X, at.Y, r/16, 0, r, 0, make([]image.Point, 0, (r*2)*(r*2))
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
		imgDrawCircle(img, col, at, r-1, true)
	}
}

// https://en.wikipedia.org/wiki/Flood_fill#Span_filling
func imgFloodFill(imgLines *image.RGBA, imgFills *image.RGBA, x int, y int) {
	img_lines := image.NewRGBA(imgLines.Bounds())
	draw.Copy(img_lines, ptZ, imgLines, imgLines.Bounds(), draw.Over, nil)

	inside := func(x int, y int) bool {
		rgba := img_lines.RGBAAt(x, y)
		return rgba.A < 123 && x >= 0 && x < img_lines.Rect.Dx() && y >= 0 && y < img_lines.Rect.Dy()
	}
	set := func(atX int, atY int) {
		if fillPixelSize <= 1 || blurSizeFactor > 0.01 {
			imgFills.Set(atX, atY, allColors[idxColSelCur])
			img_lines.Set(atX, atY, allColors[idxColSelCur])
		} else {
			img_lines.Set(atX, atY, allColors[idxColSelCur])
			for x := atX - (fillPixelSize / 2); x <= atX+(fillPixelSize/2); x++ {
				for y := atY - (fillPixelSize / 2); y <= atY+(fillPixelSize/2); y++ {
					if x >= 0 && x < img_lines.Rect.Dx() && y >= 0 && y < img_lines.Rect.Dy() {
						imgFills.Set(x, y, allColors[idxColSelCur])
					}
				}
			}
		}
	}

	if !inside(x, y) {
		return
	}
	stack := make([]int, 0, 1536)
	stack = append(stack, x, x, y, 1)
	stack = append(stack, x, x, y-1, -1)
	for len(stack) > 0 {
		popped := stack[len(stack)-4:]
		stack = stack[:len(stack)-4]
		assert(len(popped) == 4)
		x1, x2, y, dy := popped[0], popped[1], popped[2], popped[3]
		x := x1
		if inside(x, y) {
			for inside(x-1, y) {
				set(x-1, y)
				x = x - 1
			}
			if x < x1 {
				stack = append(stack, x, x1-1, y-dy, -dy)
			}
		}
		for x1 <= x2 {
			for inside(x1, y) {
				set(x1, y)
				x1 = x1 + 1
			}
			if x1 > x {
				stack = append(stack, x, x1-1, y+dy, dy)
			}
			if x1-1 > x2 {
				stack = append(stack, x2+1, x1-1, y-dy, -dy)
			}
			x1 = x1 + 1
			for x1 < x2 && !inside(x1, y) {
				x1 = x1 + 1
			}
			x = x1
		}
	}
}
