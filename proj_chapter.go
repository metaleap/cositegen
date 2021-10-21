package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Series struct {
	Name     string
	UrlName  string
	Title    map[string]string
	Desc     map[string]string
	Author   string
	Chapters []*Chapter
	Book     *Book

	parentProj *Project
}

type Chapter struct {
	Name            string
	UrlName         string
	Title           map[string]string
	Desc            map[string]string
	Author          string
	SheetsPerPage   int
	StoryboardFile  string
	GenPanelSvgText struct {
		PerLineDyCmA4 float64
		FontSizeCmA4  float64
	}

	defaultQuali int
	sheets       []*Sheet
	parentSeries *Series
	versions     []int64
	verDtLatest  struct {
		from  int64
		until int64
	}
	storyBoardPages []ChapterStoryboardPage
}

type ChapterStoryboardPage struct {
	name      string
	textBoxes []ChapterStoryboardPageTextBox
}

type ChapterStoryboardPageTextBox struct {
	xywhCm    []float64
	textSpans []string
}

func (me *Series) At(i int) fmt.Stringer { return me.Chapters[i] }
func (me *Series) Len() int              { return len(me.Chapters) }
func (me *Series) String() string        { return me.Name }

func (me *Series) numSheets() (ret int) {
	for _, chap := range me.Chapters {
		ret += len(chap.sheets)
	}
	return
}

func (me *Chapter) NextAfter(withSheetsOnly bool) *Chapter {
	series := me.parentSeries
	for now, i := false, 0; i < len(series.Chapters); i++ {
		if now && (len(series.Chapters[i].sheets) > 0 || !withSheetsOnly) {
			return series.Chapters[i]
		}
		now = (series.Chapters[i] == me)
	}
	if series.Chapters[0] == me || (withSheetsOnly && 0 == len(series.Chapters[0].sheets)) {
		return nil
	}
	return series.Chapters[0]
}

func (me *Chapter) NumPanels() (ret int) {
	for _, sheet := range me.sheets {
		n, _ := sheet.versions[0].panelCount()
		ret += n
	}
	return
}

func (me *Chapter) NumScans() (ret int) {
	for _, sheet := range me.sheets {
		ret += len(sheet.versions)
	}
	return
}

func (me *Chapter) IsSheetOnPage(pgNr int, sheetIdx int) (is bool) {
	if is = len(me.sheets) > sheetIdx; is && me.SheetsPerPage > 0 {
		is = (pgNr == (1 + (sheetIdx / me.SheetsPerPage)))
	}
	return
}

func (me *Chapter) HasBgCol() bool {
	for _, sheet := range me.sheets {
		for _, sv := range sheet.versions {
			if sv.data.hasBgCol {
				return true
			}
		}
	}
	return false
}

func (me *Chapter) PercentColorized() float64 {
	numsv, numbg := 0, 0
	for _, sheet := range me.sheets {
		for _, sv := range sheet.versions {
			if numsv++; sv.data.hasBgCol {
				numbg++
			}
		}
	}
	if numsv == 0 || numbg == 0 {
		return 0.0
	}
	return 100.0 / (float64(numsv) / float64(numbg))
}

func (me *Chapter) DateRangeOfSheets() (time.Time, time.Time) {
	var dt1, dt2 int64
	for _, sheet := range me.sheets {
		for _, sv := range sheet.versions {
			dt := sv.dateTimeUnixNano
			if dt < dt1 || dt1 == 0 {
				dt1 = dt
			}
			if dt > dt2 || dt2 == 0 {
				dt2 = dt
			}
		}
	}
	return time.Unix(0, dt1), time.Unix(0, dt2)
}

func (me *Chapter) At(i int) fmt.Stringer { return me.sheets[i] }
func (me *Chapter) Len() int              { return len(me.sheets) }
func (me *Chapter) String() string        { return me.Name }

func (me *Chapter) loadStoryboard() {
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
						if sts = trim(xmlUnesc(sts)); sts != "" {
							csptb.textSpans = append(csptb.textSpans, sts)
						}
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
