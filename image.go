package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"

	"golang.org/x/image/draw"
)

var PngEncoder = png.Encoder{CompressionLevel: png.BestCompression}

type ImgPanel struct {
	Rect    image.Rectangle
	SubRows []ImgPanel `json:",omitempty"`
	SubCols []ImgPanel `json:",omitempty"`

	Areas []ImgPanelArea `json:",omitempty"`
}

func (me *ImgPanel) HasAny(dataKey string) bool {
	for _, pta := range me.Areas {
		if pta.Data[dataKey] != "" {
			return true
		}
	}
	return false
}

type ImgPanelArea struct {
	Data map[string]string `json:",omitempty"`
	Rect image.Rectangle
}

func imgDownsized(srcImgData io.Reader, onFileDone func() error, maxWidth int) []byte {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	_ = onFileDone()

	origwidth, origheight := imgsrc.Bounds().Max.X, imgsrc.Bounds().Max.Y
	if origwidth <= maxWidth {
		return nil
	}

	newheight := int(float64(origheight) / (float64(origwidth) / float64(maxWidth)))
	imgdown := image.NewGray(image.Rect(0, 0, maxWidth, newheight))
	draw.ApproxBiLinear.Scale(imgdown, imgdown.Bounds(), imgsrc, imgsrc.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err = PngEncoder.Encode(&buf, imgdown); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// returns nil if srcImgData already consists entirely of fully black or fully white pixels
func imgToMonochrome(srcImgData io.Reader, onFileDone func() error, blackIfLessThan uint8) []byte {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	_ = onFileDone()

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
			if colbw < blackIfLessThan {
				colbw = 0
			} else {
				colbw = 255
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

func imgSubRectPng(srcImg *image.Gray, srcImgRect image.Rectangle, width *int, height *int, blackBorderSize int, whiteBorderSize int, transparent bool, gotSameSizeAsOrig *bool) []byte {
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
	*gotSameSizeAsOrig = *width > origwidth
	if *gotSameSizeAsOrig {
		*width, *height = origwidth, origheight
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
		draw.ApproxBiLinear.Scale(imgdst, imgdst.Bounds(), srcimg, srcImgRect, draw.Over, nil)
	}
	imgBwBorder(imgdst, color.Gray{255}, whiteBorderSize, 0, transparent)
	imgBwBorder(imgdst, color.Gray{0}, blackBorderSize, whiteBorderSize, transparent)
	var buf bytes.Buffer
	if err := PngEncoder.Encode(&buf, imgdst); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func imgSubRectSvg(srcImg *image.Gray, srcImgRect image.Rectangle, width int, height int, blackBorderSize int, whiteBorderSize int, transparent bool, gotSameSizeAsOrig *bool) []byte {
	pngdata := imgSubRectPng(srcImg, srcImgRect, &width, &height, blackBorderSize, whiteBorderSize, transparent, gotSameSizeAsOrig)
	svgxml := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">`
	svgxml += `<image width="` + itoa(width) + `" height="` + itoa(height) + `" xlink:href="data:image/png;base64,`
	svgxml += base64.StdEncoding.EncodeToString(pngdata)
	svgxml += `"/></svg>`
	return []byte(svgxml)
}

func imgSvgText(pta *ImgPanelArea, langId string, px1cm float64) (s string) {
	aw, ah := pta.Rect.Max.X-pta.Rect.Min.X, pta.Rect.Max.Y-pta.Rect.Min.Y
	pxfont, pxline := int(px1cm*App.Proj.Gen.PanelSvgText.FontSizeCmA4), int(px1cm*App.Proj.Gen.PanelSvgText.PerLineDyCmA4)
	s += "<svg viewbox='0 0 " + itoa(aw) + " " + itoa(ah) + "'><text x='0' y='0'>"
	for _, ln := range strings.Split(svgRepl.Replace(siteGenLocStr(pta.Data, langId)), "\n") {
		if ln == "" {
			ln = "&nbsp;"
		}
		s += "<tspan style='font-size: " + itoa(pxfont) + "px' dy='" + itoa(pxline) + "' x='0'>" + ln + "</tspan>"
	}
	s += "</text></svg>"
	return
}

func imgStitchHorizontally(fileNames []string, height int, gapWidth int, gapColor color.Color) []byte {
	totalwidth, srcimgs := 0, make(map[image.Image]int, len(fileNames))
	for _, fname := range fileNames {
		if data, err := os.ReadFile(fname); err != nil {
			panic(err)
		} else if img, _, err := image.Decode(bytes.NewReader(data)); err != nil {
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
		draw.ApproxBiLinear.Scale(dst, image.Rect(nextx, 0, nextx+width, height), img, img.Bounds(), draw.Over, nil)
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
	_ = onFileDone()
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
	cm := srcImg.Rect.Max.Y / 21

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
		prevmid := area.Min.Y
		for _, sep := range seps {
			assert(sep[1] > sep[0])
			mid := sep[0] + ((sep[1] - sep[0]) / 2)
			if mid-prevmid > cm {
				rect := image.Rect(area.Min.X, prevmid, area.Max.X, mid)
				assert(rect.In(area))
				ret = append(ret, rect)
			}
			prevmid = mid
		}
		if area.Max.Y-prevmid > cm {
			ret = append(ret, image.Rect(area.Min.X, prevmid, area.Max.X, area.Max.Y))
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
		prevmid := area.Min.X
		for _, sep := range seps {
			assert(sep[1] > sep[0])
			mid := sep[0] + ((sep[1] - sep[0]) / 2)
			if mid-prevmid > cm {
				rect := image.Rect(prevmid, area.Min.Y, mid, area.Max.Y)
				assert(rect.In(area))
				ret = append(ret, rect)
			}
			prevmid = mid
		}
		if area.Max.X-prevmid > cm {
			ret = append(ret, image.Rect(prevmid, area.Min.Y, area.Max.X, area.Max.Y))
		}
		return
	}

	me.SubCols, me.SubRows = nil, nil
	var rows, cols []image.Rectangle
	rows, cols = detectRows(me.Rect), detectCols(me.Rect)
	if len(rows) == 1 {
		rows = nil
	}
	if len(cols) == 1 {
		cols = nil
	}
	assert(!(len(rows) > 0 && len(cols) > 0))
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

func (me *ImgPanel) salvageAreasFrom(old *ImgPanel) {
	for i := 0; i < len(me.SubCols) && i < len(old.SubCols); i++ {
		me.SubCols[i].salvageAreasFrom(&old.SubCols[i])
	}
	for i := 0; i < len(me.SubRows) && i < len(old.SubRows); i++ {
		me.SubRows[i].salvageAreasFrom(&old.SubRows[i])
	}
	me.Areas = old.Areas
}
