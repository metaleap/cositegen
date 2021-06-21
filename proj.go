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
		Name     string
		Title    string
		LinkHref string
	}
	MaxImagePanelTextAreas int
	BwThreshold            uint8
	BwSmallWidth           uint16
	PageContentTexts       map[string]map[string]string
	NumSheetsInHomeBgs     int
	NumColorDistrClusters  int
	Gen                    struct {
		IdQualiList   string
		ClsSheetsPage string
		ClsSeries     string
		ClsChapter    string
		ClsPanelCols  string
		ClsPanelCol   string
		ClsPanelRows  string
		ClsPanelRow   string
		ClsPanel      string
		ClsSheet      string
		ClsSheets     string
		ClsImgHq      string
		APaging       string
		PanelSvgText  struct {
			PerLineDyCmA4 float64
			FontSizeCmA4  float64
			BoxStyle      string
			Css           map[string][]string
			AppendToFiles map[string]bool
		}
	}

	data struct {
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

	dirPath string
}

func (me *Series) At(i int) fmt.Stringer { return me.Chapters[i] }
func (me *Series) Len() int              { return len(me.Chapters) }
func (me *Series) String() string        { return me.Name }

type Chapter struct {
	Name          string
	Title         map[string]string
	SheetsPerPage int
	History       []struct {
		Date   string
		PageNr int
		Notes  map[string]string
	}

	dirPath string
	sheets  []*Sheet
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
		series.dirPath = "sheets/" + series.Name
		for _, chapter := range series.Chapters {
			chapter.dirPath = filepath.Join(series.dirPath, chapter.Name)
			sheetsdirpath := filepath.Join(chapter.dirPath, "sheets")
			files, err := os.ReadDir(sheetsdirpath)
			if err != nil {
				panic(err)
			}
			for _, f := range files {
				if fnamebase := f.Name(); !f.IsDir() {
					fname := filepath.Join(sheetsdirpath, fnamebase)
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
						sheet = &Sheet{name: sheetname}
						chapter.sheets = append(chapter.sheets, sheet)
					}

					for _, sv := range sheet.versions {
						assert(sv.name != versionname)
					}
					sheetver := &SheetVer{name: versionname, parent: sheet, fileName: fname}
					sheet.versions = append(sheet.versions, sheetver)
				}
			}
		}
	}
}
