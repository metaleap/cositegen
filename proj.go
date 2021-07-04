package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		PngDirName       string
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

func (me *Project) PercentTranslated(series *Series, langId string) float64 {
	sum, num := 0.0, 0
	for _, ser := range me.Series {
		if ser == series || series == nil {
			for _, chap := range ser.Chapters {
				if f, applicable := chap.PercentTranslated(langId, 0, -1); applicable {
					num, sum = num+1, sum+f
				}
			}
		}
	}
	if num == 0 {
		return 0
	}
	return sum / float64(num)
}

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

func (me *Chapter) PercentTranslated(langId string, pgNr int, svDt int64) (float64, bool) {
	if langId == App.Proj.Langs[0] {
		return 0, false
	}
	sum, num, allnil := 0.0, 0, true
	for i, sheet := range me.sheets {
		pgnr := 1
		if me.SheetsPerPage != 0 {
			pgnr = 1 + (i / me.SheetsPerPage)
		}
		if pgnr == pgNr || pgNr <= 0 {
			if stats := sheet.versionNoOlderThanOrLatest(svDt).percentTranslated(); stats != nil {
				allnil, sum, num = false, sum+stats[langId], num+1
			}
		}
	}
	if allnil {
		return 0, false
	}
	return sum / float64(num), true
}

type Chapter struct {
	Name          string
	UrlName       string
	Title         map[string]string
	SheetsPerPage int

	defaultQuali int
	dirPath      string
	sheets       []*Sheet
	parentSeries *Series
	versions     []int64
	verDtLatest  struct {
		from  int64
		until int64
	}
}

func (me *Chapter) At(i int) fmt.Stringer { return me.sheets[i] }
func (me *Chapter) Len() int              { return len(me.sheets) }
func (me *Chapter) String() string        { return me.Name }

func (me *Project) save() {
	jsonSave(".cache/projdata.json", &me.data)
	jsonSave("csgtexts.json", me.data.Sv.textRects)
}

func (me *Project) load() (numSheetVers int) {
	jsonLoad("cosite.json", nil, me) // exits early if no such file, before creating work dirs:
	mkDir(".cache")
	mkDir(".cache/sv")
	if fileStat(".cache/projdata.json") != nil {
		jsonLoad(".cache/projdata.json", nil, &me.data)
	}
	if fileStat("csgtexts.json") != nil {
		jsonLoad("csgtexts.json", nil, &me.data.Sv.textRects)
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
		series.parentProj, series.dirPath = me, "sheets/"+series.Name
		if series.UrlName == "" {
			series.UrlName = series.Name
		}
		for _, chapter := range series.Chapters {
			chapter.parentSeries, chapter.dirPath = series, filepath.Join(series.dirPath, chapter.Name)
			if chapter.UrlName == "" {
				chapter.UrlName = chapter.Name
			}
			files, err := os.ReadDir(chapter.dirPath)
			if err != nil {
				panic(err)
			}
			var work = struct {
				sync.WaitGroup
				sync.Mutex
			}{}
			for _, f := range files {
				if fnamebase := f.Name(); strings.HasSuffix(fnamebase, ".png") && !f.IsDir() {
					fname := filepath.Join(chapter.dirPath, fnamebase)
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
					for _, s := range chapter.sheets {
						if s.name == sheetname {
							sheet = s
							break
						}
					}
					if sheet == nil {
						sheet = &Sheet{name: sheetname, parentChapter: chapter}
						chapter.sheets = append(chapter.sheets, sheet)
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

			if len(chapter.sheets) > 0 {
				chapter.versions = []int64{0}
				for _, sheet := range chapter.sheets {
					for i, sheetver := range sheet.versions {
						if i > 0 {
							if len(chapter.versions) <= i {
								chapter.versions = append(chapter.versions, sheetver.dateTimeUnixNano)
							} else if sheetver.dateTimeUnixNano < chapter.versions[i] {
								chapter.versions[i] = sheetver.dateTimeUnixNano
							}
						} else {
							if sheetver.dateTimeUnixNano > chapter.verDtLatest.until {
								chapter.verDtLatest.until = sheetver.dateTimeUnixNano
							}
							if sheetver.dateTimeUnixNano < chapter.verDtLatest.from || chapter.verDtLatest.from == 0 {
								chapter.verDtLatest.from = sheetver.dateTimeUnixNano
							}
						}
					}
				}
			}
		}
	}
	for svid := range me.data.Sv.ById {
		if me.data.Sv.IdsToFileNames[svid] == "" {
			delete(me.data.Sv.ById, svid)
			rmDir(".cache/sv/" + svid)
		}
	}
	return
}
