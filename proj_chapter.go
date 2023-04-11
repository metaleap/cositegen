package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	GenPanelSvgText *PanelSvgTextGen
	Priv            bool
	BwThreshold     uint8

	author  *Author
	isStrip bool
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
	Storyboard       string
	GenPanelSvgText  *PanelSvgTextGen
	Priv             bool
	HomePic          []interface{}
	BwThreshold      uint8

	author       *Author
	sheets       []*Sheet
	parentSeries *Series
	isStrip      bool
	versions     []int64
	verDtLatest  struct {
		from  int64
		until int64
	}
	storyboard struct {
		fullFilePath string
		pages        []ChapterStoryboardPage
	}
}

type PanelSvgTextGen struct {
	ClsBoxPoly           string
	BoxPolyStrokeWidthCm float64
	BoxPolyDxCmA4        float64
	BoxPolyTopPx         int
	PerLineDyCmA4        float64
	FontSizeCmA4         float64
	MozScale             float64
	Css                  map[string]map[string]string
	TspanSubTagStyles    map[string]string
	TspanCssCls          string

	cssName string
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

func (me *Series) bwThreshold(dt int64) uint8 {
	if me.BwThreshold != 0 {
		return me.BwThreshold
	}
	return App.Proj.bwThreshold(dt)
}

func (me *Series) numSheets(skipPriv bool, lang string) (ret int) {
	for _, chap := range me.Chapters {
		if (chap.Priv && skipPriv) || !chap.isTransl(lang) {
			continue
		}
		ret += len(chap.sheets)
	}
	return
}

func (me *Series) numPanels(skipPriv bool, lang string) (ret int) {
	for _, chap := range me.Chapters {
		if (chap.Priv && skipPriv) || !chap.isTransl(lang) {
			continue
		}
		ret += chap.numPanels()
	}
	return
}

func (me *Series) numPages(skipPriv bool, lang string) (ret int) {
	for _, chap := range me.Chapters {
		if (chap.Priv && skipPriv) || !chap.isTransl(lang) {
			continue
		}
		ret += len(chap.SheetsPerPage)
	}
	return
}

func (me *Series) scanYearLatest(skipPriv bool, lang string) (ret int) {
	for _, chap := range me.Chapters {
		if (chap.Priv && skipPriv) || !chap.isTransl(lang) {
			continue
		}
		if year := chap.scanYearLatest(); year > ret {
			ret = year
		}
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

func (me *Chapter) bwThreshold(dt int64) uint8 {
	if me.BwThreshold != 0 {
		return me.BwThreshold
	}
	if dt == 0 {
		dt = me.verDtLatest.from
	}
	return me.parentSeries.bwThreshold(dt)
}

func (me *Chapter) nextAfter(withSheetsOnly bool) *Chapter {
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

func (me *Chapter) numPanels() (ret int) {
	for _, sheet := range me.sheets {
		n, _ := sheet.versions[0].panelCount()
		ret += n
	}
	return
}

func (me *Chapter) numScans() (ret int) {
	for _, sheet := range me.sheets {
		ret += len(sheet.versions)
	}
	return
}

func (me *Chapter) homePic() (*SheetVer, int) {
	if len(me.HomePic) > 0 {
		idxsheet := me.HomePic[0].(float64)
		idxpanel := me.HomePic[1].(float64)
		return me.sheets[int(idxsheet)].versions[0], int(idxpanel)
	}
	if len(me.sheets) > 0 && len(me.sheets[0].versions) > 0 {
		return me.sheets[0].versions[0], 0
	}
	return nil, 0
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

func (me *Chapter) isTransl(lang string) bool {
	return lang == App.Proj.Langs[0] || App.Proj.percentTranslated(lang, nil, me, nil, -1) > 50
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

func (me *Chapter) storyboardFilePath() string {
	if me.storyboard.fullFilePath == "" {
		me.storyboard.fullFilePath = filepath.Join(App.Proj.Site.StoryboardsDir, me.Storyboard)
		if info, _ := os.Stat(me.storyboard.fullFilePath); info == nil {
			me.storyboard.fullFilePath = "/"
		} else if info.IsDir() {
			fodpfilepath := filepath.Join(me.storyboard.fullFilePath, "storyboard.fodp")
			me.storyboard.fullFilePath = filepath.Join(me.storyboard.fullFilePath, "storyboard.json")
			if statfodp, statjson := fileStat(fodpfilepath), fileStat(me.storyboard.fullFilePath); statfodp != nil &&
				(statjson == nil || statfodp.ModTime().After(statjson.ModTime())) {
				_ = osExec(false, []string{"JSON_ONLY=1"}, "sbconv", fodpfilepath)
			}
			if fileStat(me.storyboard.fullFilePath) == nil {
				me.storyboard.fullFilePath = "/"
			}
		}
	}
	if me.storyboard.fullFilePath == "/" {
		return ""
	}
	return me.storyboard.fullFilePath
}

func (me *Chapter) hasBgCol() bool {
	for _, sheet := range me.sheets {
		for _, sv := range sheet.versions {
			if sv.data.hasBgCol {
				return true
			}
		}
	}
	return false
}

func (me *Chapter) percentColorized() float64 {
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

func (me *Chapter) dateRangeOfSheets(newestSheetVerOnly bool, onlyYear int) (time.Time, time.Time) {
	var dt1, dt2 int64
	for _, sheet := range me.sheets {
		for _, sv := range sheet.versions {
			dt := sv.dateTimeUnixNano
			if onlyYear > 0 && time.Unix(0, dt).Year() != onlyYear {
				continue
			}
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

func (me *PanelSvgTextGen) basedOn(base *PanelSvgTextGen) *PanelSvgTextGen {
	if me == nil {
		return nil
	}
	copy := *me
	copy.Css, copy.TspanSubTagStyles = make(map[string]map[string]string, len(me.Css)), make(map[string]string, len(me.TspanSubTagStyles))
	for k, v := range me.TspanSubTagStyles {
		copy.TspanSubTagStyles[k] = v
	}
	for k, v := range me.Css {
		m := make(map[string]string, len(v))
		for vk, vv := range v {
			m[vk] = vv
		}
		copy.Css[k] = m
	}
	if base != nil {
		copy.baseOn(base)
	}
	return &copy
}

func (me *PanelSvgTextGen) baseOn(base *PanelSvgTextGen) {
	if me.ClsBoxPoly == "" {
		me.ClsBoxPoly = base.ClsBoxPoly
	}
	if me.TspanCssCls == "" {
		me.TspanCssCls = base.TspanCssCls
	}
	if me.Css == nil {
		me.Css = make(map[string]map[string]string, len(base.Css))
	}
	for bk, bv := range base.Css {
		m := me.Css[bk]
		if m == nil {
			m = make(map[string]string, len(bv))
		}
		for k, v := range bv {
			if _, exists := m[k]; !exists {
				m[k] = v
			}
		}
		me.Css[bk] = m
	}
	if me.TspanSubTagStyles == nil {
		me.TspanSubTagStyles = make(map[string]string, len(base.TspanSubTagStyles))
	}
	for k, v := range base.TspanSubTagStyles {
		if _, exists := me.TspanSubTagStyles[k]; !exists {
			me.TspanSubTagStyles[k] = v
		}
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

func (me *Author) str(abbrev bool, forHtml bool) (ret string) {
	if me != nil {
		name := me.Name
		if abbrev {
			name = make([]string, len(me.Name))
			copy(name, me.Name)
			for i := 0; i < len(name)-1; i++ {
				name[i] = name[i][:1] + "."
			}
		}
		ret = strings.Join(name, sIf(forHtml, "&nbsp;", " "))
	}
	return
}
