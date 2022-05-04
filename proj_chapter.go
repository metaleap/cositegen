package main

import (
	"fmt"
	"strings"
	"time"
)

type StoryUrls struct {
	LinkHref   string
	DisplayUrl string
	Alt        []string
}

type Series struct {
	Name            string
	UrlName         string
	Title           map[string]string
	DescHtml        map[string]string
	Author          string
	Year            int
	StoryUrls       StoryUrls
	Chapters        []*Chapter
	Book            *BookDef
	GenPanelSvgText *PanelSvgTextGen
	Priv            bool

	author *Author
}

type Chapter struct {
	Name             string
	UrlName          string
	UrlJumpName      string
	Title            map[string]string
	TitleOrig        string
	DescHtml         map[string]string
	Author           string
	Year             int
	StoryUrls        StoryUrls
	SheetsPerPage    []int
	NumSheetsPerPage int
	StoryboardFile   string
	GenPanelSvgText  *PanelSvgTextGen
	Priv             bool
	Pic              []interface{}

	author       *Author
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

type PanelSvgTextGen struct {
	BoxPolyStrokeWidthCm float64
	ClsBoxPoly           string
	BoxPolyDxCmA4        float64
	PerLineDyCmA4        float64
	FontSizeCmA4         float64
	MozScale             float64
	Css                  map[string][]string
	AppendToFiles        map[string]bool
	TspanSubTagStyles    map[string]string
}

type Author struct {
	Name []string
}

func (me *Series) At(i int) fmt.Stringer { return me.Chapters[i] }
func (me *Series) Len() int              { return len(me.Chapters) }
func (me *Series) String() string        { return me.Name }

func (me *Series) numNonPrivChaptersWithSheets() (r int) {
	for _, chap := range me.Chapters {
		if len(chap.sheets) != 0 && !chap.Priv {
			r++
		}
	}
	return
}

func (me *Series) numSheets() (ret int) {
	for _, chap := range me.Chapters {
		ret += len(chap.sheets)
	}
	return
}

func (me *Series) scanYearHas(year int, latestSvOnly bool) bool {
	for _, chap := range me.Chapters {
		if chap.scanYearHas(year, latestSvOnly) {
			return true
		}
	}
	return false
}

func (me *Series) allSheetVersSortedByScanDate(skipPriv bool) (ret []*SheetVer) {
	for _, chapter := range me.Chapters {
		if skipPriv && chapter.Priv {
			continue
		}
		for _, sheet := range chapter.sheets {
			for _, sv := range sheet.versions {
				var added bool
				for i := range ret {
					if added = (ret[i].dateTimeUnixNano > sv.dateTimeUnixNano); added {
						ret = append(append(append(make([]*SheetVer, 0, len(ret)+1), ret[:i]...), sv), ret[i:]...)
						break
					}
				}
				if !added {
					ret = append(ret, sv)
				}
			}
		}
	}
	if skipPriv && len(ret) == 0 {
		ret = me.allSheetVersSortedByScanDate(false)
	}
	return
}

func (me *Series) dateRange() (dtOldest int64, dtNewest int64) {
	for _, chap := range me.Chapters {
		for _, sheet := range chap.sheets {
			dt := sheet.versions[0].dateTimeUnixNano
			if dtOldest == 0 || dt < dtOldest {
				dtOldest = dt
			}
			if dtNewest == 0 || dt > dtNewest {
				dtNewest = dt
			}
		}
	}
	return
}

func (me *Chapter) NextAfter(withSheetsOnly bool) *Chapter {
	series := me.parentSeries
	for ok, i := false, 0; i < len(series.Chapters); i++ {
		if ok && (len(series.Chapters[i].sheets) > 0 || !withSheetsOnly) {
			return series.Chapters[i]
		}
		ok = (series.Chapters[i] == me)
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

func (me *Chapter) scanYearLatest() (r int) {
	for _, sheet := range me.sheets {
		dt := time.Unix(0, sheet.versions[0].dateTimeUnixNano)
		if dtyear := dt.Year(); dtyear > r {
			r = dtyear
		}
	}
	return
}
func (me *Chapter) scanYearHas(year int, latestSvOnly bool) bool {
	for _, sheet := range me.sheets {
		for _, sv := range sheet.versions {
			if dt := time.Unix(0, sv.dateTimeUnixNano); dt.Year() == year {
				return true
			}
			if latestSvOnly {
				break
			}
		}
	}
	return false
}

func (me *Chapter) isSheetOnPgNr(pgNr int, sheetIdx int) (is bool) {
	return pgNr == (1 + me.pgIdxOfSheet(sheetIdx))
}

func (me *Chapter) pgIdxOfSheet(sheetIdx int) int {
	var shidx int
	for i, numsheets := range me.SheetsPerPage {
		if sheetIdx >= shidx && sheetIdx < (shidx+numsheets) {
			return i
		}
		shidx += numsheets
	}
	panic(sheetIdx)
}

func (me *Chapter) ensureSheetsPerPage() {
	if len(me.SheetsPerPage) == 0 {
		if sum := 0; me.NumSheetsPerPage == 0 || me.NumSheetsPerPage > len(me.sheets) {
			me.SheetsPerPage = []int{len(me.sheets)}
		} else {
			me.SheetsPerPage = make([]int, (len(me.sheets)%me.NumSheetsPerPage)+(len(me.sheets)/me.NumSheetsPerPage))
			for i := 0; i < len(me.SheetsPerPage); i++ {
				me.SheetsPerPage[i], sum = me.NumSheetsPerPage, sum+me.NumSheetsPerPage
			}
			for i := len(me.SheetsPerPage) - 1; sum > len(me.sheets); {
				me.SheetsPerPage[i], sum = me.SheetsPerPage[i]-1, sum-1
			}
		}
	}
}

func (me *Chapter) readDurationMinutes() int {
	return len(me.sheets) / 2
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

func (me *Chapter) dateRangeOfSheets(newestSheetVerOnly bool) (time.Time, time.Time) {
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
			if newestSheetVerOnly {
				break
			}
		}
	}
	return time.Unix(0, dt1), time.Unix(0, dt2)
}

func (me *Chapter) At(i int) fmt.Stringer { return me.sheets[i] }
func (me *Chapter) Len() int              { return len(me.sheets) }
func (me *Chapter) String() string        { return me.Name }

func (me *PanelSvgTextGen) mergeWithParent(base *PanelSvgTextGen) {
	if me.ClsBoxPoly == "" {
		me.ClsBoxPoly = base.ClsBoxPoly
	}
	if me.Css == nil {
		me.Css = base.Css
	}
	if me.AppendToFiles == nil {
		me.AppendToFiles = base.AppendToFiles
	}
	if me.TspanSubTagStyles == nil {
		me.TspanSubTagStyles = base.TspanSubTagStyles
	}
	for ptr, val := range map[*float64]float64{
		&me.BoxPolyStrokeWidthCm: base.BoxPolyStrokeWidthCm,
		&me.BoxPolyDxCmA4:        base.BoxPolyDxCmA4,
		&me.PerLineDyCmA4:        base.PerLineDyCmA4,
		&me.FontSizeCmA4:         base.FontSizeCmA4,
		&me.MozScale:             base.MozScale,
	} {
		if *ptr < 0.01 { // float64 `==0.0`...
			*ptr = val
		}
	}
}

func (me *Author) String(abbrev bool) (ret string) {
	if me != nil {
		name := me.Name
		if abbrev {
			name = make([]string, len(me.Name))
			copy(name, me.Name)
			for i := 0; i < len(name)-1; i++ {
				name[i] = name[i][:1] + "."
			}
		}
		ret = strings.Join(name, "&nbsp;")
	}
	return
}
