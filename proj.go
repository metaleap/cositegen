package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Project struct {
	Title  string
	Desc   map[string]string
	Series []*Series
	Langs  []string
	Qualis []struct {
		Name     string
		SizeHint int
	}
	AtomFile struct {
		PubDates    []string
		Name        string
		Title       string
		LinkHref    string
		ContentHtml map[string]string
	}
	MaxImagePanelTextAreas int
	BwThreshold            uint8
	BwSmallWidth           uint16
	PanelBorderCm          float64
	PageContentTexts       map[string]map[string]string
	NumSheetsInHomeBgs     int
	NumColorDistrClusters  int
	DirModes               struct {
		Ltr, Rtl struct {
			Name  string
			Title map[string]string
			Desc  map[string]string
		}
	}
	Gen struct {
		IdQualiList      string
		ClsViewerPage    string
		ClsNonViewerPage string
		ClsSeries        string
		ClsChapter       string
		ClsPanelCol      string
		ClsPanelRow      string
		ClsPanel         string
		ClsViewer        string
		ClsSheetsView    string
		ClsRowsView      string
		ClsSheet         string
		ClsImgHq         string
		APaging          string
		ImgSrcLang       string
		PicDirName       string
		PanelSvgText     struct {
			BoxPolyStrokeWidthCm float64
			ClsBoxPoly           string
			BoxPolyDxCmA4        float64
			PerLineDyCmA4        float64
			FontSizeCmA4         float64
			Css                  map[string][]string
			AppendToFiles        map[string]bool
		}
	}

	allPrepsDone bool
	data         struct {
		Sv struct {
			fileNamesToIds map[string]string
			IdsToFileNames map[string]string
			ById           map[string]*SheetVerData

			textRects map[string][][]ImgPanelArea
		}
		PngOpt map[string][]string
	}
}

func (me *Project) At(i int) fmt.Stringer { return me.Series[i] }
func (me *Project) Len() int              { return len(me.Series) }

type Series struct {
	Name     string
	UrlName  string
	Title    map[string]string
	Desc     map[string]string
	Author   string
	Chapters []*Chapter

	dirPath    string
	parentProj *Project
}

func (me *Series) At(i int) fmt.Stringer { return me.Chapters[i] }
func (me *Series) Len() int              { return len(me.Chapters) }
func (me *Series) String() string        { return me.Name }

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

type Chapter struct {
	Name           string
	UrlName        string
	Title          map[string]string
	SheetsPerPage  int
	StoryboardFile string

	defaultQuali int
	dirPath      string
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

func (me *Chapter) At(i int) fmt.Stringer { return me.sheets[i] }
func (me *Chapter) Len() int              { return len(me.sheets) }
func (me *Chapter) String() string        { return me.Name }

func (me *Project) save() {
	jsonSave(".cache/data.json", &me.data)
	jsonSave("texts.json", me.data.Sv.textRects)
}

func (me *Project) load() (numSheetVers int) {
	jsonLoad("comicsite.json", nil, me) // exits early if no such file, before creating work dirs:
	mkDir(".cache")
	if fileStat(".cache/data.json") != nil {
		jsonLoad(".cache/data.json", nil, &me.data)
	}
	if fileStat("texts.json") != nil {
		jsonLoad("texts.json", nil, &me.data.Sv.textRects)
	} else {
		me.data.Sv.textRects = map[string][][]ImgPanelArea{}
	}
	me.data.Sv.fileNamesToIds = map[string]string{}
	me.data.Sv.IdsToFileNames = map[string]string{}
	if me.data.Sv.ById == nil {
		me.data.Sv.ById = map[string]*SheetVerData{}
	}
	if me.data.PngOpt == nil {
		me.data.PngOpt = map[string][]string{}
	}

	for _, series := range me.Series {
		series.parentProj, series.dirPath = me, "scans/"+series.Name
		if series.UrlName == "" {
			series.UrlName = series.Name
		}
		for _, chap := range series.Chapters {
			chap.parentSeries, chap.dirPath = series, filepath.Join(series.dirPath, chap.Name)
			if chap.UrlName == "" {
				chap.UrlName = chap.Name
			}
			files, err := os.ReadDir(chap.dirPath)
			if err != nil {
				panic(err)
			}
			var work = struct {
				sync.WaitGroup
				sync.Mutex
			}{}
			for _, f := range files {
				if fnamebase := f.Name(); strings.HasSuffix(fnamebase, ".png") && !f.IsDir() {
					fname := filepath.Join(chap.dirPath, fnamebase)
					fnamebase = fnamebase[:len(fnamebase)-len(".png")]
					versionname := fnamebase[1+strings.LastIndexByte(fnamebase, '.'):]
					t, _ := time.Parse("20060102", versionname)
					dt := t.UnixNano()
					if dt <= 0 {
						printLn("SkipWip: " + fname)
						continue
					}
					sheetname := fnamebase[:strings.LastIndexByte(fnamebase, '.')]
					if sheetname == "" {
						panic("invalid sheet-file name: " + fname)
					}

					var sheet *Sheet
					for _, s := range chap.sheets {
						if s.name == sheetname {
							sheet = s
							break
						}
					}
					if sheet == nil {
						sheet = &Sheet{name: sheetname, parentChapter: chap}
						chap.sheets = append(chap.sheets, sheet)
					}
					sheetver := &SheetVer{dateTimeUnixNano: dt, parentSheet: sheet, fileName: fname}
					sheet.versions = append([]*SheetVer{sheetver}, sheet.versions...)
					numSheetVers++

					work.Add(1)
					go func(sv *SheetVer) {
						data := fileRead(sv.fileName)
						sv.id = contentHashStr(data)
						work.Lock()
						App.Proj.data.Sv.fileNamesToIds[sv.fileName] = sv.id
						App.Proj.data.Sv.IdsToFileNames[sv.id] = sv.fileName
						work.Unlock()
						sv.data = App.Proj.data.Sv.ById[sv.id]
						work.Done()
					}(sheetver)
				}
			}
			work.Wait()

			if len(chap.sheets) > 0 {
				chap.versions = []int64{0}
				for _, sheet := range chap.sheets {
					for i, sheetver := range sheet.versions {
						if i > 0 {
							if len(chap.versions) <= i {
								chap.versions = append(chap.versions, sheetver.dateTimeUnixNano)
							} else if sheetver.dateTimeUnixNano < chap.versions[i] {
								chap.versions[i] = sheetver.dateTimeUnixNano
							}
						} else {
							if sheetver.dateTimeUnixNano > chap.verDtLatest.until {
								chap.verDtLatest.until = sheetver.dateTimeUnixNano
							}
							if sheetver.dateTimeUnixNano < chap.verDtLatest.from || chap.verDtLatest.from == 0 {
								chap.verDtLatest.from = sheetver.dateTimeUnixNano
							}
						}
					}
				}
				if fileStat(chap.StoryboardFile) != nil {
					chap.loadStoryboard()
				} else if chap.StoryboardFile != "" {
					panic(chap.StoryboardFile)
				}
			}
		}
	}
	for svid := range me.data.Sv.ById {
		if me.data.Sv.IdsToFileNames[svid] == "" {
			delete(me.data.Sv.ById, svid)
			rmDir(".cache/" + svid)
		}
	}
	return
}

func (me *Project) hasSvgQuali() bool {
	for _, q := range me.Qualis {
		if q.SizeHint == 0 {
			return true
		}
	}
	return false
}

func (me *Project) percentTranslated(lang string, ser *Series, chap *Chapter, sheetVer *SheetVer, pgNr int) float64 {
	numtotal, numtrans := 0, 0
	for _, series := range me.Series {
		if ser != nil && ser != series {
			continue
		}
		for _, chapter := range series.Chapters {
			if chap != nil && chap != chapter {
				continue
			}
			for i, sheet := range chapter.sheets {
				if pgNr > 0 && !chapter.IsSheetOnPage(pgNr, i) {
					continue
				}
				for _, sv := range sheet.versions {
					if sheetVer != nil && sheetVer != sv {
						continue
					}
					for _, areas := range App.Proj.data.Sv.textRects[sv.id] {
						for _, area := range areas {
							if def := trim(area.Data[App.Proj.Langs[0]]); def != "" {
								if numtotal++; trim(area.Data[lang]) != "" {
									numtrans++
								}
							}
						}
					}
				}
			}
		}
	}
	if numtotal == 0 {
		return -1.0
	}
	return float64(numtrans) * (100.0 / float64(numtotal))
}

func (me *Chapter) loadStoryboard() {
	s := string(fileRead(me.StoryboardFile))

	for _, sp := range xmlInners(s, `<draw:page>`, `</draw:page>`) {
		csp := ChapterStoryboardPage{name: xmlAttr(sp, "draw:name")}
		for _, sf := range xmlInners(sp, `<draw:frame>`, `</draw:frame>`) {
			csptb := ChapterStoryboardPageTextBox{}
			for _, attr := range xmlAttrs(sf, "svg:x", "svg:y", "svg:width", "svg:height") {
				if f, err := strconv.ParseFloat(strings.TrimSuffix(attr, "cm"), 64); err != nil || !strings.HasSuffix(attr, "cm") {
					panic(attr)
				} else {
					csptb.xywhCm = append(csptb.xywhCm, f)
				}
			}
			assert(len(csptb.xywhCm) == 4)
			for itb, stb := range xmlInners(sf, "<draw:text-box>", "</draw:text-box>") {
				if itb > 0 {
					panic(sf)
				}
				for itp, stp := range xmlInners(stb, "<text:p>", "</text:p>") {
					if itp > 0 {
						panic(stb)
					}
					for _, sts := range xmlInners(stp, "<text:span>", "</text:span>") {
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
