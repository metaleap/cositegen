package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	data         struct {
		ContentHashes map[string]string
		SheetVer      map[string]*SheetVerData

		sheetVerPanelAreas map[string][][]ImgPanelArea
	}
}

func (me *Project) At(i int) fmt.Stringer { return me.Series[i] }
func (me *Project) Len() int              { return len(me.Series) }

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
	jsonSave(".csg/projdata.json", &me.data)
	jsonSave(".csg/panelareas.json", me.data.sheetVerPanelAreas)
}

func (me *Project) load() {
	jsonLoad("cosite.json", nil, me) // exits early if no such file, before creating work dirs:
	rmDir(".csg/tmp")
	mkDir(".csg")
	mkDir(".csg/tmp")
	mkDir(".csg/projdata")
	if _, err := os.Stat(".csg/projdata.json"); err == nil {
		jsonLoad(".csg/projdata.json", nil, &me.data)
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if _, err := os.Stat(".csg/panelareas.json"); err == nil {
		jsonLoad(".csg/panelareas.json", nil, &me.data.sheetVerPanelAreas)
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if me.data.sheetVerPanelAreas == nil {
		me.data.sheetVerPanelAreas = map[string][][]ImgPanelArea{}
	}
	if me.data.ContentHashes == nil {
		me.data.ContentHashes = map[string]string{}
	}
	if me.data.SheetVer == nil {
		me.data.SheetVer = map[string]*SheetVerData{}
	}

	for filename := range me.data.sheetVerPanelAreas {
		if fileinfo, err := os.Stat(filename); err != nil || fileinfo.IsDir() {
			delete(me.data.sheetVerPanelAreas, filename)
		}
	}
	for filename, contenthash := range me.data.ContentHashes {
		if fileinfo, err := os.Stat(filename); err != nil || fileinfo.IsDir() {
			delete(me.data.SheetVer, contenthash)
			rmDir(".csg/projdata/" + contenthash)
			delete(me.data.ContentHashes, filename)
		}
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
					sheet.versions = append([]*SheetVer{sheetver}, sheet.versions...)
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
}
