package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			PerLineDyCmA4        float64
			FontSizeCmA4         float64
			BoxPolyStrokeWidthCm float64
			ClsBoxPoly           string
			Css                  map[string][]string
			AppendToFiles        map[string]bool
		}
	}

	allPrepsDone bool
	svData       struct {
		fileNamesToIds map[string]string
		IdsToFileNames map[string]string
		ById           map[string]*SheetVerData

		textRects map[string][][]ImgPanelArea
	}
}

func (me *Project) At(i int) fmt.Stringer { return me.Series[i] }
func (me *Project) Len() int              { return len(me.Series) }

func (me *Project) PercentTranslated(series *Series, langId string) float64 {
	sum, num := 0.0, 0
	for _, ser := range me.Series {
		if ser == series || series == nil {
			for _, chap := range ser.Chapters {
				if f, applicable := chap.PercentTranslated(langId, 0, ""); applicable {
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
	if series.Chapters[0] == me {
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

func (me *Chapter) DateRangeOfSheets() (string, string) {
	var dt1, dt2 int64
	for _, sheet := range me.sheets {
		for _, sv := range sheet.versions {
			sdt := sv.data.DateTimeUnixNano
			if sdt < dt1 || dt1 == 0 {
				dt1 = sdt
			}
			if sdt > dt2 || dt2 == 0 {
				dt2 = sdt
			}
		}
	}
	return time.Unix(0, dt1).Format("2006-01-02"), time.Unix(0, dt2).Format("2006-01-02")
}

func (me *Chapter) PercentTranslated(langId string, pgNr int, svName string) (float64, bool) {
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
			if stats := sheet.versionNamedOrLatest(svName).percentTranslated(); stats != nil {
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
	Title         map[string]string
	SheetsPerPage int

	defaultQuali  int
	dirPath       string
	sheets        []*Sheet
	parentSeries  *Series
	sheetVerNames []string
}

func (me *Chapter) At(i int) fmt.Stringer { return me.sheets[i] }
func (me *Chapter) Len() int              { return len(me.sheets) }
func (me *Chapter) String() string        { return me.Name }

func (me *Project) save() {
	jsonSave(".csg/svdata.json", &me.svData)
	jsonSave("csgtexts.json", me.svData.textRects)
}

func (me *Project) load() (numSheetVers int) {
	jsonLoad("cosite.json", nil, me) // exits early if no such file, before creating work dirs:
	rmDir(".csg/tmp")
	mkDir(".csg")
	mkDir(".csg/tmp")
	mkDir(".csg/sv")
	if _, err := os.Stat(".csg/svdata.json"); err == nil {
		jsonLoad(".csg/svdata.json", nil, &me.svData)
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if _, err := os.Stat("csgtexts.json"); err == nil {
		jsonLoad("csgtexts.json", nil, &me.svData.textRects)
	} else if !os.IsNotExist(err) {
		panic(err)
	} else {
		me.svData.textRects = map[string][][]ImgPanelArea{}
	}
	me.svData.fileNamesToIds = map[string]string{}
	me.svData.IdsToFileNames = map[string]string{}
	if me.svData.ById == nil {
		me.svData.ById = map[string]*SheetVerData{}
	}

	for _, series := range me.Series {
		series.parentProj = me
		series.dirPath = "sheets/" + series.Name
		for _, chapter := range series.Chapters {
			chapter.parentSeries = series
			chapter.dirPath = filepath.Join(series.dirPath, chapter.Name)
			files, err := os.ReadDir(chapter.dirPath)
			if err != nil {
				panic(err)
			}
			for _, f := range files {
				if fnamebase := f.Name(); !f.IsDir() {
					fname := filepath.Join(chapter.dirPath, fnamebase)
					fnamebase = fnamebase[:len(fnamebase)-len(filepath.Ext(fnamebase))]
					versionname := fnamebase[strings.LastIndexByte(fnamebase, '_')+1:]
					if versionname == fnamebase {
						panic("invalid sheet-file name: " + fname)
					} else if versionname == "" {
						printLn("SkipWip: " + fname)
						continue
					}
					sheetname := fnamebase[:strings.LastIndexByte(fnamebase, '_')]
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

					for _, sv := range sheet.versions {
						assert(sv.name != versionname)
					}
					sheetver := &SheetVer{name: versionname, parentSheet: sheet, fileName: fname}
					sheetver.load()
					sheet.versions = append([]*SheetVer{sheetver}, sheet.versions...)
					numSheetVers++
					foundinchapter := false
					for _, svname := range chapter.sheetVerNames {
						if foundinchapter = (svname == sheetver.name); foundinchapter {
							break
						}
					}
					if !foundinchapter {
						chapter.sheetVerNames = append([]string{sheetver.name}, chapter.sheetVerNames...)
					}
				}
			}
		}
	}
	for svid := range me.svData.ById {
		if me.svData.IdsToFileNames[svid] == "" {
			delete(me.svData.ById, svid)
			rmDir(".csg/sv/" + svid)
		}
	}
	return
}
