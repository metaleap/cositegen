package main

import (
	"fmt"
	"strings"
	"time"
)

type Series struct {
	Name            string
	UrlName         string
	Title           map[string]string
	DescHtml        map[string]string
	Author          string
	Year            int
	StoryUrls       []string
	Chapters        []*Chapter
	Book            *BookDef
	GenPanelSvgText *PanelSvgTextGen
	Priv            bool

	author *Author
}

type Chapter struct {
	Name            string
	UrlName         string
	UrlJumpName     string
	Title           map[string]string
	DescHtml        map[string]string
	Author          string
	Year            int
	StoryUrls       []string
	SheetsPerPage   int
	StoryboardFile  string
	GenPanelSvgText *PanelSvgTextGen
	Priv            bool

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

func (me *Series) numSheets() (ret int) {
	for _, chap := range me.Chapters {
		ret += len(chap.sheets)
	}
	return
}

func (me *Series) allSheetVersSortedByScanDate() (ret []*SheetVer) {
	for _, chapter := range me.Chapters {
		for _, sheet := range chapter.sheets {
			for _, sv := range sheet.versions {
				var added bool
				for i := range ret {
					if added = ret[i].dateTimeUnixNano > sv.dateTimeUnixNano; added {
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

func (me *Author) fullName() (ret string) {
	if me != nil {
		ret = strings.Join(me.Name, " ")
	}
	return
}
