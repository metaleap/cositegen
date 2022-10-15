package main

import (
	"image"
	"image/color"
	"io"
	"strings"
)

const panelMinDiv = 11

var DeNewLineRepl = strings.NewReplacer("\n", " ")
var svgRepl *strings.Replacer
var svgTxtCounter int

type ImgPanel struct {
	Rect              image.Rectangle
	SubRows           []ImgPanel `json:",omitempty"`
	SubCols           []ImgPanel `json:",omitempty"`
	recenteredXOffset int
}

type ImgPanelArea struct {
	Data                  map[string]string `json:",omitempty"`
	SvgTextTransformAttr  string            `json:",omitempty"`
	SvgTextTspanStyleAttr string            `json:",omitempty"`
	PointTo               *image.Point      `json:",omitempty"`
	Rect                  image.Rectangle
}

func imgPanels(srcImgData io.Reader, onDecoded func() error) ImgPanel {
	imgsrc, _, err := image.Decode(srcImgData)
	if onDecoded != nil {
		_ = onDecoded() // allow early file-closing for the caller
	}
	if err != nil {
		panic(err)
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
	panelmin := srcImg.Rect.Max.Y / panelMinDiv // ~1.9cm
	brborder := int(4.0 * (float64(srcImg.Rect.Max.Y) / 210.0))

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
			if blackrect := image.Rect(area.Min.X, sep[0]+brborder, area.Max.X, sep[1]-brborder); ((sep[1] - sep[0]) > 2*panelmin) && imgIsRectFullyOfColor(srcImg, blackrect, color.Gray{0}) {
				ret = append(ret, blackrect)
			}
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
			if blackrect := image.Rect(sep[0]+brborder, area.Min.Y, sep[1]-brborder, area.Max.Y); ((sep[1] - sep[0]) > 2*panelmin) && imgIsRectFullyOfColor(srcImg, blackrect, color.Gray{0}) {
				ret = append(ret, blackrect)
			}
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
		imgpanel = imgpanel.flattened()
		me.SubRows = append(me.SubRows, imgpanel)
	}
	for _, col := range cols {
		imgpanel := ImgPanel{Rect: col}
		imgpanel.detectSubPanels(srcImg)
		imgpanel = imgpanel.flattened()
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

func (me *ImgPanel) setTopLevelRowRecenteredX(root *ImgPanel, w int, h int) {
	padding := bookPanelsHPadding
start:
	var netwidth int
	for _, col := range me.SubCols {
		netwidth += col.Rect.Dx() + padding
	}
	netwidth -= padding
	if netwidth > w {
		padding--
		goto start
	}

	x := (w - netwidth) / 2
	me.recenteredXOffset = x // - root.Rect.Min.X
	for i := range me.SubCols {
		offset := x - me.SubCols[i].Rect.Min.X
		me.SubCols[i].setRecenteredXOffset(offset)
		x = x + me.SubCols[i].Rect.Dx() + padding
	}
}

func (me *ImgPanel) setRecenteredXOffset(offset int) {
	if len(me.SubCols) > 0 {
		for i := range me.SubCols {
			me.SubCols[i].setRecenteredXOffset(offset)
		}
	}
	if len(me.SubRows) > 0 {
		for i := range me.SubRows {
			me.SubRows[i].setRecenteredXOffset(offset)
		}
	}
	me.recenteredXOffset = offset
}

func (me *ImgPanel) nextPanel(parent *Chapter) (foundSheet *SheetVer, foundPanel *ImgPanel, pIdx int, pgNr int) {
	pastme, pgnr, pgnrme := false, 0, 0
	for i, sheet := range parent.sheets {
		if foundPanel != nil {
			break
		}
		pgnr = 1 + parent.pgIdxOfSheet(i)
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

func (me *SheetVer) imgSvgText(pidx int, tidx int, pta *ImgPanelArea, langId string, lineX int, fontSizeCmA4 float64, perLineDyCmA4 float64, forHtml bool, forEbook bool) (s string) {
	isstorytitle := (pta.SvgTextTspanStyleAttr == "_storytitle")
	if svgRepl == nil {
		repls := []string{
			" ", "&nbsp;",
			"&lt;/i&gt;", "</tspan>",
			"&lt;/b&gt;", "</tspan>",
			"&lt;/u&gt;", "</tspan>",
			"&lt;i&gt;", "<tspan class='i' font-style='italic'>",
			"&lt;b&gt;", "<tspan class='b' font-weight='bold'>",
			"&lt;u&gt;", "<tspan class='u' text-decoration='underline'>",
		}
		for _, tagname := range sortedMapKeys(me.parentSheet.parentChapter.GenPanelSvgText.TspanSubTagStyles) {
			repls = append(repls,
				"&lt;"+tagname+"&gt;", "<tspan style='"+me.parentSheet.parentChapter.GenPanelSvgText.TspanSubTagStyles[tagname]+"'>",
				"&lt;/"+tagname+"&gt;", "</tspan>",
			)
		}
		svgRepl = strings.NewReplacer(repls...)
	}

	pxfont, pxline := int(me.data.PxCm*fontSizeCmA4), int(me.data.PxCm*perLineDyCmA4)
	svgTxtCounter++

	if forHtml {
		s += sIf(isstorytitle, ``, `<use id='_t_`+itoa(svgTxtCounter)+`' xlink:href="t.`+me.parentSheet.parentChapter.parentSeries.Name+`.`+me.parentSheet.parentChapter.Name+`.`+langId+`.svg#`+me.id+`_`+itoa(pidx)+`t`+itoa(tidx+1)+`"/>`)
	} else {
		mozscale := me.parentSheet.parentChapter.GenPanelSvgText.MozScale > 0.01 && !forEbook
		if mozscale {
			s += `<svg class="mz" width="` + itoa(me.data.PanelsTree.Rect.Dx()) + `">`
		}
		s += "<text style='font-size: " + itoa(pxfont) + "px;' transform='" + trim(DeNewLineRepl.Replace(pta.SvgTextTransformAttr)) + "'>"
		ts := "<tspan style='" + trim(DeNewLineRepl.Replace(pta.SvgTextTspanStyleAttr)) + "'" + sIf(isstorytitle || strings.Contains(pta.SvgTextTspanStyleAttr, "font-family"), "", " class='std'") + ">"
		svgcss := me.parentSheet.parentChapter.GenPanelSvgText
		for _, ln := range strings.Split(svgRepl.Replace(hEsc(locStr(pta.Data, langId))), hEscs['\n']) {
			if ln == "" {
				ln = "&nbsp;"
			}
			ln += hEscs['\n']
			ts += "<tspan dy='" + itoa(pxline) + "' x='" + itoa(lineX) + "'"
			if svgcss.TspanCssCls != "" {
				ts += " class='" + svgcss.TspanCssCls + "'"
			}
			ts += ">" + ln + "</tspan>"
		}
		ts += "</tspan>"
		s += ts + "</text>"
		if mozscale {
			s += "</svg>"
		}
		s = htmlEscdToXmlEsc(s)
	}
	return
}
