package main

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
)

type ImgPanel struct {
	Rect  image.Rectangle
	Areas []struct {
		Rect image.Rectangle
		Data map[string]string
	} `json:",omitempty"`
	SubRows []ImgPanel `json:",omitempty"`
	SubCols []ImgPanel `json:",omitempty"`
}

// returns nil if srcImgData already smaller than maxWidth
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

	imgdown := image.NewGray(image.Rect(0, 0, maxWidth, int(float64(origheight)/(float64(origwidth)/float64(maxWidth)))))
	draw.ApproxBiLinear.Scale(imgdown, imgdown.Bounds(), imgsrc, imgsrc.Bounds(), draw.Over, nil)
	var pngbuf bytes.Buffer
	if err = png.Encode(&pngbuf, imgdown); err != nil {
		panic(err)
	}
	return pngbuf.Bytes()
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
	if err = png.Encode(&pngbuf, imggray); err != nil {
		panic(err)
	}
	return pngbuf.Bytes()
}

func imgPanels(srcImgData io.Reader, onFileDone func() error) ImgPanel {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	_ = onFileDone()
	ret := ImgPanel{Rect: imgsrc.Bounds()}
	ret.detectSubPanels(imgsrc.(*image.Gray), true, true)
	return ret
}

func (me *ImgPanel) detectSubPanels(srcImg *image.Gray, findRows bool, findCols bool) {
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
		return
	}

	me.SubCols, me.SubRows = nil, nil
	var rows, cols []image.Rectangle
	if findRows {
		rows = detectRows(me.Rect)
	}
	if findCols {
		cols = detectCols(me.Rect)
	}
	if len(rows) == 1 {
		rows = nil
	}
	if len(cols) == 1 {
		cols = nil
	}
	assert(!(len(rows) > 0 && len(cols) > 0))
	for _, row := range rows {
		imgpanel := ImgPanel{Rect: row}
		imgpanel.detectSubPanels(srcImg, false, true)
		me.SubRows = append(me.SubRows, imgpanel)
	}
	for _, col := range cols {
		imgpanel := ImgPanel{Rect: col}
		imgpanel.detectSubPanels(srcImg, true, false)
		me.SubCols = append(me.SubCols, imgpanel)
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
