package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-forks/gopnm"
	"golang.org/x/image/draw"
)

var PngEncoder = png.Encoder{CompressionLevel: png.BestCompression}
var ImgScaler draw.Interpolator = draw.CatmullRom
var DeNewLineRepl = strings.NewReplacer("\n", " ")
var svgRepl *strings.Replacer

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
		minwidth, minheight := img.Rect.Max.X, img.Rect.Max.Y
		for y := img.Rect.Max.Y - 1; y >= 0 && minheight == img.Rect.Max.Y; y-- {
			rowbrightsum := 0
			for x := 0; x < img.Rect.Max.X; x++ {
				rowbrightsum += int(img.GrayAt(x, y).Y)
			}
			if rowbright := (rowbrightsum / img.Rect.Max.X); rowbright < threshold {
				minheight = y
			}
		}
		for x := 0; x < img.Rect.Max.X && minwidth == img.Rect.Max.X; x++ {
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

func imgSvgToPng(svgFilePath string, pngFilePath string, repl *strings.Replacer, reSize int, dpi int, noTmpFile bool, onDone func()) {
	if onDone != nil {
		defer onDone()
	}
	svgdata := fileRead(svgFilePath)
	chash := contentHashStr(svgdata)
	tmpfilepath := ".ccache/.svgpng/" + chash + "." + itoa(reSize) + ".png"
	if noTmpFile {
		tmpfilepath = pngFilePath
	}
	if fileStat(tmpfilepath) == nil || noTmpFile {
		if repl != nil {
			svgFilePath += ".fix.svg"
			fileWrite(svgFilePath, []byte(repl.Replace(string(svgdata))))
		}
		cmdargs := []string{svgFilePath,
			"-quality", "90", /*png max lossless compression*/
			"-background", "white",
			"-alpha", "remove",
			"-alpha", "off"}
		if dpi != 0 {
			cmdargs = append(cmdargs,
				"-units", "PixelsPerInch",
				"-set", "units", "PixelsPerInch",
				"-density", itoa(dpi))
		}
		if reSize != 0 {
			cmdargs = append(cmdargs, "-resize", itoa(reSize))
		}
		_ = osExec(true, "convert", append(cmdargs, tmpfilepath)...)
	}
	if !noTmpFile {
		fileLinkOrCopy(tmpfilepath, pngFilePath)
	}
}

func imgDownsized(srcImgData io.Reader, onFileDone func() error, maxWidth int, transparent bool) []byte {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	if onFileDone != nil {
		_ = onFileDone()
	}

	origwidth, origheight := imgsrc.Bounds().Max.X, imgsrc.Bounds().Max.Y
	if origwidth <= maxWidth && !transparent {
		return nil
	}

	if transparent {
		img := image.NewNRGBA(imgsrc.Bounds())
		for x := 0; x < imgsrc.Bounds().Max.X; x++ {
			for y := 0; y < imgsrc.Bounds().Max.Y; y++ {
				gray := imgsrc.(*image.Gray).GrayAt(x, y)
				img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 255 - gray.Y})
			}
		}
		imgsrc = img
	}

	newheight := int(float64(origheight) / (float64(origwidth) / float64(maxWidth)))
	var imgdown draw.Image
	if transparent {
		imgdown = image.NewNRGBA(image.Rect(0, 0, maxWidth, newheight))
	} else {
		imgdown = image.NewGray(image.Rect(0, 0, maxWidth, newheight))
	}
	ImgScaler.Scale(imgdown, imgdown.Bounds(), imgsrc, imgsrc.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err = PngEncoder.Encode(&buf, imgdown); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func imgFill(img draw.Image, r image.Rectangle, c color.Color) {
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			img.Set(x, y, c)
		}
	}
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

func imgSubRectSvg(srcImg *image.Gray, srcImgRect image.Rectangle, blackBorderSize int) (ret []byte) {
	if blackBorderSize != 0 {
		imgDrawRect(srcImg, srcImgRect, blackBorderSize, 0)
	}
	var buf bytes.Buffer
	name := strconv.FormatInt(time.Now().UnixNano(), 36)
	pnmpath, svgpath := "/dev/shm/"+name+".pbm", "/dev/shm/"+name+".svg"
	if err := pnm.Encode(&buf, srcImg.SubImage(srcImgRect), pnm.PBM); err != nil {
		panic(err)
	}
	fileWrite(pnmpath, buf.Bytes())
	osExec(true, "potrace", "-s", pnmpath, "-o", svgpath)
	ret = fileRead(svgpath)
	_, _ = os.Remove(pnmpath), os.Remove(svgpath)
	return
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
	origwidth, origheight := srcImgRect.Dx(), srcImgRect.Dy()
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

var svgTxtCounter int

func (me *SheetVer) imgSvgText(pidx int, tidx int, pta *ImgPanelArea, langId string, lineX int, fontSizeCmA4 float64, perLineDyCmA4 float64, forHtml bool, forEbook bool) (s string) {
	if svgRepl == nil {
		repls := []string{
			" ", "&nbsp;",
			"&lt;/i&gt;", "</tspan>",
			"&lt;/b&gt;", "</tspan>",
			"&lt;/u&gt;", "</tspan>",
			"&lt;i&gt;", "<tspan style='font-style: italic'>",
			"&lt;b&gt;", "<tspan style='font-weight: bolder'>",
			"&lt;u&gt;", "<tspan style='text-decoration: underline'>",
		}
		for tagname, style := range App.Proj.Gen.PanelSvgText.TspanSubTagStyles {
			repls = append(repls,
				"&lt;"+tagname+"&gt;", "<tspan style='"+style+"'>",
				"&lt;/"+tagname+"&gt;", "</tspan>",
			)
		}
		svgRepl = strings.NewReplacer(repls...)
	}

	pxfont, pxline := int(me.data.PxCm*fontSizeCmA4), int(me.data.PxCm*perLineDyCmA4)
	svgTxtCounter++

	if forHtml {
		s += "<text id='_w_" + itoa(svgTxtCounter) + "' style='visibility: hidden; font-size: " + itoa(pxfont) + "px'><tspan><tspan dy='" + itoa(pxline) + "' x='" + itoa(lineX) + "'>&#9881;...</tspan></tspan><title><tspan>Loading... / Wird geladen...</tspan></title></text>"
		s += `<use id='_t_` + itoa(svgTxtCounter) + `' xlink:href="t.` + me.parentSheet.parentChapter.parentSeries.Name + `.` + me.parentSheet.parentChapter.Name + `.` + langId + `.svg#` + me.id + `_` + itoa(pidx) + `t` + itoa(tidx+1) + `"/>`
	} else {
		mozscale := me.parentSheet.parentChapter.GenPanelSvgText.MozScale > 0.01 && !forEbook
		if mozscale {
			s += `<svg class="mz" width="` + itoa(me.data.PanelsTree.Rect.Dx()) + `">`
		}
		s += "<text style='font-size: " + itoa(pxfont) + "px;' transform='" + trim(DeNewLineRepl.Replace(pta.SvgTextTransformAttr)) + "'>"
		ts := "<tspan style='" + trim(DeNewLineRepl.Replace(pta.SvgTextTspanStyleAttr)) + "'>"
		for _, ln := range strings.Split(svgRepl.Replace(hEsc(locStr(pta.Data, langId))), hEscs['\n']) {
			if ln == "" {
				ln = "&nbsp;"
			}
			ln += hEscs['\n']
			ts += "<tspan dy='" + itoa(pxline) + "' x='" + itoa(lineX) + "'>" + ln + "</tspan>"
			if tripleoffset := 0; !forHtml {
				if forEbook {
					tripleoffset = 2
				}
				if tripleoffset != 0 {
					ts += "<tspan dy='" + itoa(-tripleoffset) + "' x='" + itoa(lineX-tripleoffset) + "'>" + ln + "</tspan>"
					ts += "<tspan dy='" + itoa(tripleoffset+tripleoffset) + "' x='" + itoa(lineX+tripleoffset) + "'>" + ln + "</tspan>"
				}
			}
		}
		ts += "</tspan>"
		s += ts /*+ "<title>" + ts + "</title>"*/ + "</text>"
		if mozscale {
			s += "</svg>"
		}
	}
	if forHtml {
		s += "<script>vHide('_t_" + itoa(svgTxtCounter) + "');vShow('_w_" + itoa(svgTxtCounter) + "');</script>"
	} else {
		s = htmlEscdToXmlEsc(s)
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
		data := fileRead(fname)
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
		imgFill(dst, image.Rect(0, 0, totalwidth, height), gapColor)
	}
	nextx := gapWidth / 2
	for img, width := range srcimgs {
		dr := image.Rect(nextx, 0, nextx+width, height)
		imgFill(dst, dr, color.NRGBA{255, 255, 255, 255})
		ImgScaler.Scale(dst, dr, img, img.Bounds(), draw.Over, nil)
		nextx += width + gapWidth
	}

	var buf bytes.Buffer
	if err := PngEncoder.Encode(&buf, dst); err != nil {
		panic(err)
	}
	return buf.Bytes()
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
	panelmin := srcImg.Rect.Max.X / 10 // min. ~2.9+cm

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
			if assert(rect.In(area)); rect.Dy() > panelmin {
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
			if assert(rect.In(area)); rect.Dx() > panelmin {
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
