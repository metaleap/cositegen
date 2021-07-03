package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"

	"github.com/go-forks/gopnm"
	"golang.org/x/image/draw"
)

var PngEncoder = png.Encoder{CompressionLevel: png.BestCompression}
var ImgScaler draw.Interpolator = draw.CatmullRom
var DeNewLineRepl = strings.NewReplacer("\n", " ")
var svgRepl = strings.NewReplacer(
	" ", "&nbsp;",
	"<b>", "<tspan class='b'>",
	"<u>", "<tspan class='u'>",
	"<i>", "<tspan class='i'>",
	"</b>", "</tspan>",
	"</u>", "</tspan>",
	"</i>", "</tspan>",
)

type ImgPanel struct {
	Rect    image.Rectangle
	SubRows []ImgPanel `json:",omitempty"`
	SubCols []ImgPanel `json:",omitempty"`
}

type ImgPanelArea struct {
	Data                  map[string]string `json:",omitempty"`
	SvgTextTransformAttr  string            `json:",omitempty"`
	SvgTextTspanStyleAttr string            `json:",omitempty"`
	PointTo               *image.Point      `json:",omitempty"`
	Rect                  image.Rectangle
}

func imgPnmToPng(srcImgData io.ReadCloser, dstImgFile io.WriteCloser, ensureWide bool, snipLeftAndBottomEdges bool) {
	srcimg, err := pnm.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	_ = srcImgData.Close()

	dstbounds := srcimg.(*image.Gray).Bounds() // the cast as an assert, not as a need
	if ensureWide && dstbounds.Max.X < dstbounds.Max.Y {
		dstbounds.Max.X, dstbounds.Max.Y = dstbounds.Max.Y, dstbounds.Max.X
		dstimg, srcbounds := image.NewGray(dstbounds), srcimg.Bounds()
		for dstx := 0; dstx < dstbounds.Max.X; dstx++ {
			for dsty := 0; dsty < dstbounds.Max.Y; dsty++ {
				srcx, srcy := dsty, (srcbounds.Max.Y-1)-dstx
				dstimg.Set(dstx, dsty, srcimg.At(srcx, srcy))
			}
		}
		srcimg = dstimg
	}
	if threshold, img := 96, srcimg.(*image.Gray); snipLeftAndBottomEdges {
		var minwidth, minheight int
		for y := img.Rect.Max.Y - 1; y >= 0 && minheight == 0; y-- {
			rowbrightsum := 0
			for x := 0; x < img.Rect.Max.X; x++ {
				rowbrightsum += int(img.GrayAt(x, y).Y)
			}
			if rowbright := (rowbrightsum / img.Rect.Max.X); rowbright < threshold {
				minheight = y
			}
		}
		for x := 0; x < img.Rect.Max.X && minwidth == 0; x++ {
			colbrightsum := 0
			for y := 0; y < minheight; y++ {
				colbrightsum += int(img.GrayAt(x, y).Y)
			}
			if colbright := (colbrightsum / minheight); colbright < threshold {
				minwidth = img.Rect.Max.X - x
			}
		}
		dstw, dsth := minwidth, minheight
		imgdst := image.NewGray(image.Rect(0, 0, dstw, dsth))
		imgDrawRect(imgdst, imgdst.Rect, 472, 0)
		ddx, ddy, dsx := (dstw-minwidth)/2, (dsth-minheight)/2, (img.Rect.Max.X - minwidth)
		for x := 0; x < minwidth; x++ {
			for y := 0; y < minheight; y++ {
				col := img.GrayAt(dsx+x, y)
				imgdst.SetGray(x+ddx, y+ddy, col)
			}
		}
		imgDrawRect(imgdst, imgdst.Rect, 44, 0)
		srcimg = imgdst
	}
	if err := PngEncoder.Encode(dstImgFile, srcimg); err != nil {
		panic(err)
	}
	_ = dstImgFile.Close()
}

func imgDownsized(srcImgData io.Reader, onFileDone func() error, maxWidth int) []byte {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	if onFileDone != nil {
		_ = onFileDone()
	}

	origwidth, origheight := imgsrc.Bounds().Max.X, imgsrc.Bounds().Max.Y
	if origwidth <= maxWidth {
		return nil
	}

	newheight := int(float64(origheight) / (float64(origwidth) / float64(maxWidth)))
	imgdown := image.NewGray(image.Rect(0, 0, maxWidth, newheight))
	ImgScaler.Scale(imgdown, imgdown.Bounds(), imgsrc, imgsrc.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err = PngEncoder.Encode(&buf, imgdown); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func imgGrayDistrs(srcImgData io.Reader, onFileDone func() error, numClusters int) (r []int) {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	if onFileDone != nil {
		_ = onFileDone()
	}

	r = make([]int, numClusters)
	m := 256.0 / float64(numClusters)
	for px := 0; px < imgsrc.Bounds().Max.X; px++ {
		for py := 0; py < imgsrc.Bounds().Max.Y; py++ {
			var cm uint8 // ensure grayscale
			switch colsrc := imgsrc.At(px, py).(type) {
			case color.Gray:
				cm = colsrc.Y
			case color.RGBA:
				cm = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			case color.NRGBA:
				cm = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			default:
				panic(colsrc)
			}
			r[int(float64(cm)/m)]++
		}
	}
	return
}

// returns nil if srcImgData already consists entirely of fully black or fully white pixels
func imgToMonochrome(srcImgData io.Reader, onFileDone func() error, blackIfLessThan uint8) []byte {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	if onFileDone != nil {
		_ = onFileDone()
	}

	allbw, imggray := true, image.NewGray(image.Rect(0, 0, imgsrc.Bounds().Max.X, imgsrc.Bounds().Max.Y))
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
			}
			if !(colbw == 255 || colbw == 0) {
				allbw = false
			}

			// now black&white-only
			if blackIfLessThan > 0 {
				if colbw < blackIfLessThan {
					colbw = 0
				} else {
					colbw = 255
				}
			}

			imggray.Set(px, py, color.Gray{Y: colbw})
		}
	}

	if allbw {
		return nil
	}

	var pngbuf bytes.Buffer
	if err = PngEncoder.Encode(&pngbuf, imggray); err != nil {
		panic(err)
	}
	return pngbuf.Bytes()
}

func imgBwBorder(imgdst draw.Image, bwColor color.Gray, size int, offset int, transparent bool) {
	if size > 0 {
		var col color.Color = bwColor
		if transparent {
			col = color.NRGBA{R: 0, G: 0, B: 0, A: 255 - bwColor.Y}
		}
		for px := imgdst.Bounds().Min.X + offset; px < (imgdst.Bounds().Max.X - offset); px++ {
			for i := 0; i < size; i++ {
				imgdst.Set(px, imgdst.Bounds().Min.Y+i+offset, col)
				imgdst.Set(px, imgdst.Bounds().Max.Y-(i+1+offset), col)
			}
		}
		for py := imgdst.Bounds().Min.Y + offset; py < imgdst.Bounds().Max.Y-offset; py++ {
			for i := 0; i < size; i++ {
				imgdst.Set(imgdst.Bounds().Min.X+i+offset, py, col)
				imgdst.Set(imgdst.Bounds().Max.X-(i+1+offset), py, col)
			}
		}
	}
}

func imgSubRectPng(srcImg *image.Gray, srcImgRect image.Rectangle, width *int, height *int, blackBorderSize int, transparent bool, gotSameSizeAsOrig *bool) []byte {
	imgdst := imgSubRect(srcImg, srcImgRect, width, height, blackBorderSize, transparent, gotSameSizeAsOrig)
	var buf bytes.Buffer
	if err := PngEncoder.Encode(&buf, imgdst); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func imgSubRect(srcImg *image.Gray, srcImgRect image.Rectangle, width *int, height *int, blackBorderSize int, transparent bool, gotSameSizeAsOrig *bool) image.Image {
	origwidth, origheight := srcImgRect.Max.X-srcImgRect.Min.X, srcImgRect.Max.Y-srcImgRect.Min.Y
	assert(((*width < origwidth) == (*height < origheight)) &&
		((*width > origwidth) == (*height > origheight)))

	var srcimg draw.Image = srcImg
	if transparent {
		srcimg = image.NewNRGBA(image.Rect(0, 0, srcImg.Bounds().Max.X, srcImg.Bounds().Max.Y))
		for px := srcImgRect.Min.X; px < srcImgRect.Max.X; px++ {
			for py := srcImgRect.Min.Y; py < srcImgRect.Max.Y; py++ {
				col := srcImg.GrayAt(px, py)
				srcimg.Set(px, py, color.NRGBA{R: 0, G: 0, B: 0, A: 255 - col.Y})
			}
		}
	}

	var imgdst draw.Image
	if *width > origwidth {
		*gotSameSizeAsOrig, *width, *height = true, origwidth, origheight
		if !transparent {
			imgdst = srcImg.SubImage(srcImgRect).(draw.Image)
		} else {
			imgdst = srcimg.(*image.NRGBA).SubImage(srcImgRect).(draw.Image)
		}
	} else {
		imgdst = image.NewGray(image.Rect(0, 0, *width, *height))
		if transparent {
			imgdst = image.NewNRGBA(image.Rect(0, 0, *width, *height))
		}
		ImgScaler.Scale(imgdst, imgdst.Bounds(), srcimg, srcImgRect, draw.Over, nil)
	}
	imgBwBorder(imgdst, color.Gray{0}, blackBorderSize, 0, transparent)
	return imgdst
}

func imgSvgText(pta *ImgPanelArea, langId string, px1cm float64, wrapInSvgTag bool, lineX int) (s string) {
	aw, ah := pta.Rect.Max.X-pta.Rect.Min.X, pta.Rect.Max.Y-pta.Rect.Min.Y
	pxfont, pxline := int(px1cm*App.Proj.Gen.PanelSvgText.FontSizeCmA4), int(px1cm*App.Proj.Gen.PanelSvgText.PerLineDyCmA4)
	s += "<text " + /*"x='0' y='0' "+*/ "style='font-size: " + itoa(pxfont) + "px' " + "transform='" + trim(DeNewLineRepl.Replace(pta.SvgTextTransformAttr)) + "'>"
	s += "<tspan style='" + trim(DeNewLineRepl.Replace(pta.SvgTextTspanStyleAttr)) + "'>"
	for _, ln := range strings.Split(svgRepl.Replace(locStr(pta.Data, langId)), "\n") {
		if ln == "" {
			ln = "&nbsp;"
		}
		s += "<tspan dy='" + itoa(pxline) + "' x='" + itoa(lineX) + "'>" + ln + "</tspan>"
	}
	s += "</tspan></text>"
	if wrapInSvgTag {
		s = "<svg viewbox='0 0 " + itoa(aw) + " " + itoa(ah) + "'>" + s + "</svg>"
	}
	return
}

func imgDrawRect(imgDst *image.Gray, rect image.Rectangle, thickness int, gray uint8) {
	for x := rect.Min.X; x < rect.Max.X; x++ {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			if x < (rect.Min.X+thickness) || x > (rect.Max.X-thickness) ||
				y < (rect.Min.Y+thickness) || y > (rect.Max.Y-thickness) {
				imgDst.SetGray(x, y, color.Gray{gray})
			}
		}
	}
}

func imgStitchHorizontally(fileNames []string, height int, gapWidth int, gapColor color.Color) []byte {
	totalwidth, srcimgs := 0, make(map[image.Image]int, len(fileNames))
	for _, fname := range fileNames {
		data := readFile(fname)
		if img, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			panic(err)
		} else {
			width := int(float64(img.Bounds().Max.X) / (float64(img.Bounds().Max.Y) / float64(height)))
			srcimgs[img] = width
			totalwidth += width + gapWidth
		}
	}
	var dst draw.Image
	if _, isbw := gapColor.(color.Gray); isbw || gapWidth <= 0 {
		dst = image.NewGray(image.Rect(0, 0, totalwidth, height))
	} else {
		dst = image.NewNRGBA(image.Rect(0, 0, totalwidth, height))
	}
	if gapWidth > 0 {
		for x := 0; x < totalwidth; x++ {
			for y := 0; y < height; y++ {
				dst.Set(x, y, gapColor)
			}
		}
	}
	nextx := gapWidth / 2
	for img, width := range srcimgs {
		ImgScaler.Scale(dst, image.Rect(nextx, 0, nextx+width, height), img, img.Bounds(), draw.Over, nil)
		nextx += width + gapWidth
	}

	var buf bytes.Buffer
	if err := PngEncoder.Encode(&buf, dst); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func imgVectorizeToSvg(srcImg *image.Gray, rect image.Rectangle) []byte {
	return nil
}

func imgPanels(srcImgData io.Reader, onFileDone func() error) ImgPanel {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	if onFileDone != nil {
		_ = onFileDone()
	}
	ret := ImgPanel{Rect: imgsrc.Bounds()}
	ret.detectSubPanels(imgsrc.(*image.Gray))
	return ret.flattened()
}

func (me ImgPanel) flattened() ImgPanel {
	for i := range me.SubRows {
		me.SubRows[i] = me.SubRows[i].flattened()
	}
	for i := range me.SubCols {
		me.SubCols[i] = me.SubCols[i].flattened()
	}
	if len(me.SubCols) == 1 && len(me.SubRows) == 0 {
		return me.SubCols[0]
	} else if len(me.SubCols) == 0 && len(me.SubRows) == 1 {
		return me.SubRows[0]
	}
	return me
}

func (me *ImgPanel) detectSubPanels(srcImg *image.Gray) {
	panelmin := srcImg.Rect.Max.X / 15 // ~min. 1.9cm

	detectRows := func(area image.Rectangle) (ret []image.Rectangle) {
		laststart, seps := -1, [][2]int{}
		for py := area.Min.Y; py < area.Max.Y; py++ {
			isfullsep := true
			for px := area.Min.X; px < area.Max.X; px++ {
				if col := srcImg.At(px, py).(color.Gray); col.Y != 0 {
					isfullsep = false
					break
				}
			}
			if isfullsep && laststart == -1 {
				laststart = py
			} else if (!isfullsep) && laststart != -1 {
				seps = append(seps, [2]int{laststart, py})
				laststart = -1
			}
		}
		if laststart != -1 {
			seps = append(seps, [2]int{laststart, area.Max.Y})
		}
		prev := area.Min.Y
		for _, sep := range seps {
			assert(sep[1] > sep[0])
			rect := image.Rect(area.Min.X, prev, area.Max.X, sep[0])
			if assert(rect.In(area)); (rect.Max.Y - rect.Min.Y) > panelmin {
				ret = append(ret, rect)
			}
			prev = sep[1]
		}
		if area.Max.Y-prev > panelmin {
			ret = append(ret, image.Rect(area.Min.X, prev, area.Max.X, area.Max.Y))
		}
		return
	}

	detectCols := func(area image.Rectangle) (ret []image.Rectangle) {
		laststart, seps := -1, [][2]int{}
		for px := area.Min.X; px < area.Max.X; px++ {
			isfullsep := true
			for py := area.Min.Y; py < area.Max.Y; py++ {
				if col := srcImg.At(px, py).(color.Gray); col.Y != 0 {
					isfullsep = false
					break
				}
			}
			if isfullsep && laststart == -1 {
				laststart = px
			} else if (!isfullsep) && laststart != -1 {
				seps = append(seps, [2]int{laststart, px})
				laststart = -1
			}
		}
		if laststart != -1 {
			seps = append(seps, [2]int{laststart, area.Max.X})
		}
		prev := area.Min.X
		for _, sep := range seps {
			assert(sep[1] > sep[0])
			rect := image.Rect(prev, area.Min.Y, sep[0], area.Max.Y)
			if assert(rect.In(area)); (rect.Max.X - rect.Min.X) > panelmin {
				ret = append(ret, rect)
			}
			prev = sep[1]
		}
		if (area.Max.X - prev) > panelmin {
			ret = append(ret, image.Rect(prev, area.Min.Y, area.Max.X, area.Max.Y))
		}
		return
	}

	me.SubCols, me.SubRows = nil, nil
	var rows, cols []image.Rectangle
	if rows = detectRows(me.Rect); len(rows) <= 1 {
		if rows, cols = nil, detectCols(me.Rect); len(cols) <= 1 {
			cols = nil
		}
	}
	for _, row := range rows {
		imgpanel := ImgPanel{Rect: row}
		imgpanel.detectSubPanels(srcImg)
		me.SubRows = append(me.SubRows, imgpanel.flattened())
	}
	for _, col := range cols {
		imgpanel := ImgPanel{Rect: col}
		imgpanel.detectSubPanels(srcImg)
		me.SubCols = append(me.SubCols, imgpanel.flattened())
	}
}

func (me *ImgPanel) iter(onPanel func(*ImgPanel)) {
	assert(len(me.SubCols) == 0 || len(me.SubRows) == 0)
	if len(me.SubRows) > 0 {
		for i := range me.SubRows {
			me.SubRows[i].iter(onPanel)
		}
	} else if len(me.SubCols) > 0 {
		for i := range me.SubCols {
			me.SubCols[i].iter(onPanel)
		}
	} else {
		onPanel(me)
	}
}

func (me *ImgPanel) nextPanel(parent *Chapter) (foundSheet *SheetVer, foundPanel *ImgPanel, pIdx int, pgNr int) {
	pastme, pgnr, pgnrme := false, 0, 0
	for i, sheet := range parent.sheets {
		if foundPanel != nil {
			break
		} else if parent.SheetsPerPage == 0 {
			pgnr = 1
		} else if (i % parent.SheetsPerPage) == 0 {
			pgnr++
		}
		assert(len(sheet.versions) == 1)
		for _, sv := range sheet.versions {
			_ = sv.ensurePrep(false, false)
			if sv.data != nil && sv.data.PanelsTree != nil {
				pidx := 0
				sv.data.PanelsTree.iter(func(panel *ImgPanel) {
					if panel == me {
						pastme, pgnrme = true, pgnr
					} else if pastme && foundPanel == nil &&
						(panel.Rect.Min.X != me.Rect.Min.X || pgnrme != pgnr) {
						foundPanel, foundSheet, pIdx, pgNr = panel, sv, pidx, pgnr
					}
					pidx++
				})
				if foundPanel != nil {
					break
				}
			}
		}
	}
	return
}
