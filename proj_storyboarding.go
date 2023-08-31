package main

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/metaleap/cositegen/sb_shared"
)

type ChapterStoryboardPage struct {
	name        string
	canDualLang bool
	textBoxes   []ChapterStoryboardPageTextBox
	panels      []Object
}

type ChapterStoryboardPageTextBox struct {
	xywhCm    []float64
	textSpans [][]string
}

func (me *Chapter) loadStoryboard() {
	switch filepath.Ext(me.storyboardFilePath()) {
	case ".json":
		me.loadStoryboardJson()
	case ".fodp":
		me.loadStoryboardFodp()
	}
}

func (me *Chapter) loadStoryboardJson() {
	var sb Storyboard
	jsonLoad(me.storyboardFilePath(), nil, &sb)

	for _, page := range sb {
		if len(page.Panels) == 0 && len(page.Balloons) == 0 {
			continue
		}
		pg := ChapterStoryboardPage{name: page.Name, canDualLang: true, panels: page.Panels}
		for _, txt := range page.Balloons {
			pg.textBoxes = append(pg.textBoxes, ChapterStoryboardPageTextBox{
				xywhCm:    []float64{txt.CmX, txt.CmY, txt.CmW, txt.CmH},
				textSpans: [][]string{txt.Paras},
			})
		}
		me.storyboard.pages = append(me.storyboard.pages, pg)
	}
}

func (me *Chapter) loadStoryboardFodp() {
	s := strings.Replace(string(fileRead(me.storyboardFilePath())), "<text:s/>", "", -1)
	for _, sp := range xmlOuters(s, `<draw:page>`, `</draw:page>`) {
		csp := ChapterStoryboardPage{name: xmlAttr(sp, "draw:name")}
		for _, sf := range xmlOuters(sp, `<draw:frame>`, `</draw:frame>`) {
			csptb := ChapterStoryboardPageTextBox{}
			for _, attr := range xmlAttrs(sf, "svg:x", "svg:y", "svg:width", "svg:height") {
				if f, err := strconv.ParseFloat(strings.TrimSuffix(attr, "cm"), 64); err != nil || !strings.HasSuffix(attr, "cm") {
					panic(attr)
				} else {
					csptb.xywhCm = append(csptb.xywhCm, f)
				}
			}
			assert(len(csptb.xywhCm) == 4)
			for itb, stb := range xmlOuters(sf, "<draw:text-box>", "</draw:text-box>") {
				if itb > 0 {
					panic(sf)
				}
				for _, stp := range xmlOuters(stb, "<text:p>", "</text:p>") {
					for _, sts := range xmlOuters(stp, "<text:span>", "</text:span>") {
						sts = sts[:strings.LastIndexByte(sts, '<')]
						sts = sts[strings.LastIndexByte(sts, '>')+1:]
						textspans := []string{}
						if sts = trim(xmlUnesc(sts)); sts != "" {
							textspans = append(textspans, sts)
						}
						csptb.textSpans = [][]string{textspans}
					}
				}
			}
			if len(csptb.textSpans) > 0 {
				csp.textBoxes = append(csp.textBoxes, csptb)
			}
		}
		if len(csp.textBoxes) > 0 {
			me.storyboard.pages = append(me.storyboard.pages, csp)
		}
	}
}

func (me *Chapter) panelsTreeFromStoryboard(sv *SheetVer) *ImgPanel {
	me.loadStoryboard()
	page := &me.storyboard.pages[indexOf(me.sheets, sv.parentSheet)]

	img_bw, _, err := image.Decode(bytes.NewReader(fileRead(sv.data.bwFilePath)))
	if err != nil {
		panic(err)
	}
	img := image.NewGray(img_bw.Bounds())
	imgFill(img, img.Bounds(), color.Gray{Y: 255})
	cmW, cmH := 0.0, 0.0
	for _, pnl := range page.panels {
		if x := pnl.CmX + pnl.CmW; x > cmW {
			cmW = x
		}
		if y := pnl.CmY + pnl.CmH; y > cmH {
			cmH = y
		}
	}
	pxCmX, pxCmY := float64(img.Bounds().Max.X)/cmW, float64(img.Bounds().Max.Y)/cmH
	for _, pnl := range page.panels {
		rect := image.Rect(int(pnl.CmX*pxCmX), int(pnl.CmY*pxCmY), int((pnl.CmX+pnl.CmW)*pxCmX), int((pnl.CmY+pnl.CmH)*pxCmY))
		imgDrawRect(img, rect, App.Proj.Sheets.Panel.TreeFromStoryboard.BorderInner, 0)
	}
	imgDrawRect(img, img.Bounds(), App.Proj.Sheets.Panel.TreeFromStoryboard.BorderOuter, 0)

	// data := pngEncode(img)
	// fileWrite(sv.data.bwFilePath+".pnls.png", data)

	ret := imgPanels(img)
	// find any completely white (thus non-existing and falsely-"detected") panels and remove them
	var remove_panels []*ImgPanel
	ret.each(func(pnl *ImgPanel) {
		rect := image.Rect(pnl.Rect.Min.X+App.Proj.Sheets.Panel.TreeFromStoryboard.BorderInner, pnl.Rect.Min.Y+App.Proj.Sheets.Panel.TreeFromStoryboard.BorderInner, pnl.Rect.Max.X-App.Proj.Sheets.Panel.TreeFromStoryboard.BorderInner, pnl.Rect.Max.Y-App.Proj.Sheets.Panel.TreeFromStoryboard.BorderInner)
		if imgIsRectFullyOfColor(img_bw.(*image.Gray), rect, color.Gray{Y: 255}) {
			remove_panels = append(remove_panels, pnl)
		}
	})
	ret.removePanels(remove_panels...)
	return ret
}

func (me *ChapterStoryboardPage) dualLangTextBoxes() (ret []ChapterStoryboardPageTextBox) {
	ret = me.textBoxes
	if me.canDualLang {
		ret = make([]ChapterStoryboardPageTextBox, 0, len(ret)/2)
		got := map[int]bool{}
		for i, tbi := range me.textBoxes {
			var overlaps []int
			if got[i] {
				continue
			}
			got[i] = true
			for j, tbj := range me.textBoxes {
				if i != j {
					recti := image.Rect(int(tbi.xywhCm[0]*10.0), int(tbi.xywhCm[1]*10.0), int((tbi.xywhCm[0]+tbi.xywhCm[2])*10.0), int((tbi.xywhCm[1]+tbi.xywhCm[3])*10.0))
					rectj := image.Rect(int(tbj.xywhCm[0]*10.0), int(tbj.xywhCm[1]*10.0), int((tbj.xywhCm[0]+tbj.xywhCm[2])*10.0), int((tbj.xywhCm[1]+tbj.xywhCm[3])*10.0))
					if recti.Overlaps(rectj) {
						overlaps = append(overlaps, j)
					}
				}
			}
			if len(overlaps) > 1 {
				panic(itoa(len(overlaps)) + " OVERLAPS for:\t" + strings.Join(tbi.textSpans[0], " "))
			}
			tb := ChapterStoryboardPageTextBox{
				xywhCm:    append(make([]float64, 0, 4), tbi.xywhCm...), // copy
				textSpans: append([][]string{}, tbi.textSpans...),       // copy
			}
			if len(overlaps) > 0 {
				got[overlaps[0]] = true
				tb.xywhCm[0] = math.Min(tb.xywhCm[0], me.textBoxes[overlaps[0]].xywhCm[0])
				tb.xywhCm[1] = math.Min(tb.xywhCm[1], me.textBoxes[overlaps[0]].xywhCm[1])
				tb.xywhCm[2] = math.Max(tb.xywhCm[2], me.textBoxes[overlaps[0]].xywhCm[2])
				tb.xywhCm[3] = math.Max(tb.xywhCm[3], me.textBoxes[overlaps[0]].xywhCm[3])
				tb.textSpans = append(tb.textSpans, me.textBoxes[overlaps[0]].textSpans...)
			}
			ret = append(ret, tb)
		}
	}
	return
}
