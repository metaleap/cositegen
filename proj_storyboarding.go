package main

import (
	"image"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

type ChapterStoryboardPage struct {
	name        string
	canDualLang bool
	textBoxes   []ChapterStoryboardPageTextBox
}

type ChapterStoryboardPageTextBox struct {
	xywhCm    []float64
	textSpans [][]string
}

func (me *Chapter) loadStoryboard() {
	switch filepath.Ext(me.StoryboardFile) {
	case ".json":
		me.loadStoryboardJson()
	case ".fodp":
		me.loadStoryboardFodp()
	}
}

func (me *Chapter) loadStoryboardJson() {
	// BEGIN SHARED TYPES: keep in sync with ../sb*/types.go
	type SizeAndPos struct {
		CmW float64
		CmH float64
		CmX float64
		CmY float64
	}
	type Object struct {
		SizeAndPos
		Paras []string
	}
	type Page struct {
		Name     string
		Balloons []Object
		Panels   []Object
	}
	type Storyboard []Page
	// END SHARED TYPES

	var sb Storyboard
	jsonLoad(me.StoryboardFile, nil, &sb)

	for _, page := range sb {
		pg := ChapterStoryboardPage{name: page.Name, canDualLang: true}
		for _, txt := range page.Balloons {
			pg.textBoxes = append(pg.textBoxes, ChapterStoryboardPageTextBox{
				xywhCm:    []float64{txt.CmX, txt.CmY, txt.CmW, txt.CmH},
				textSpans: [][]string{txt.Paras},
			})
		}
		me.storyBoardPages = append(me.storyBoardPages, pg)
	}
}

func (me *Chapter) loadStoryboardFodp() {
	s := strings.Replace(string(fileRead(me.StoryboardFile)), "<text:s/>", "", -1)
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
			me.storyBoardPages = append(me.storyBoardPages, csp)
		}
	}
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
