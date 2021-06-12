package main

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
)

type ImgPanel struct {
	Rect    image.Rectangle
	SubRows []ImgPanel `json:",omitempty"`
	SubCols []ImgPanel `json:",omitempty"`
}

func (me *ImgPanel) detectSubPanels(srcImg *image.Gray) {
	cm := srcImg.Rect.Max.Y / 21
	var detectRows, detectCols func(image.Rectangle) []image.Rectangle

	detectRows = func(area image.Rectangle) (ret []image.Rectangle) {
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
		var prevmid int
		for _, sep := range seps {
			mid := sep[0] + ((sep[1] - sep[0]) / 2)
			if mid-prevmid > cm {
				ret = append(ret, image.Rect(area.Min.X, prevmid, area.Max.X, mid))
			}
			prevmid = mid
		}
		return
	}

	detectCols = func(area image.Rectangle) []image.Rectangle {
		for px := area.Min.X; px < area.Max.X; px++ {
		}
		return nil
	}

	me.SubCols, me.SubRows = nil, nil
	rows, cols := detectRows(me.Rect), detectCols(me.Rect)
	if len(rows) > 0 && len(cols) > 0 {
		panic("ASSERT")
	}
	for _, row := range rows {
		me.SubRows = append(me.SubRows, ImgPanel{Rect: row})
	}
	for _, col := range cols {
		me.SubCols = append(me.SubCols, ImgPanel{Rect: col})
	}
}

func imgPanels(srcImgData io.Reader, onFileDone func() error) ImgPanel {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	_ = onFileDone()
	ret := ImgPanel{Rect: imgsrc.Bounds()}
	ret.detectSubPanels(imgsrc.(*image.Gray))
	return ret
}

func imgToMonochrome(srcImgData io.Reader, onFileDone func() error, blackIfLessThan uint8) []byte {
	imgsrc, _, err := image.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	_ = onFileDone()

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
