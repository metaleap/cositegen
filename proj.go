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
	PageContentTexts       map[string]map[string]string
	NumSheetsInHomeBgs     int
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
		ClsPanelArea  string
		ClsSheet      string
		ClsSheets     string
		ClsImgHq      string
		APaging       string
		PanelSvgText  struct {
			PerLineDyCmA4 float64
			FontSizeCmA4  float64
			Css           map[string][]string
			AppendToFiles map[string]bool
		}
	}

	meta struct {
		ContentHashes map[string]string
		SheetVer      map[string]*SheetVerMeta

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
	jsonSave(".csg/meta.json", &me.meta)
	jsonSave(".csg/panelareas.json", me.meta.sheetVerPanelAreas)
}

func (me *Project) load() {
	jsonLoad("cosite.json", nil, me) // exits early if no such file, before creating work dirs:
	rmDir(".csg/pnm")
	mkDir(".csg")
	mkDir(".csg/pnm")
	mkDir(".csg/meta")
	if _, err := os.Stat(".csg/meta.json"); err == nil {
		jsonLoad(".csg/meta.json", nil, &me.meta)
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if _, err := os.Stat(".csg/panelareas.json"); err == nil {
		jsonLoad(".csg/panelareas.json", nil, &me.meta.sheetVerPanelAreas)
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if me.meta.sheetVerPanelAreas == nil {
		me.meta.sheetVerPanelAreas = map[string][][]ImgPanelArea{}
	}
	if me.meta.ContentHashes == nil {
		me.meta.ContentHashes = map[string]string{}
	}
	if me.meta.SheetVer == nil {
		me.meta.SheetVer = map[string]*SheetVerMeta{}
	}

	for filename := range me.meta.sheetVerPanelAreas {
		if fileinfo, err := os.Stat(filename); err != nil || fileinfo.IsDir() {
			delete(me.meta.sheetVerPanelAreas, filename)
		}
	}
	for filename, contenthash := range me.meta.ContentHashes {
		if fileinfo, err := os.Stat(filename); err != nil || fileinfo.IsDir() {
			delete(me.meta.SheetVer, contenthash)
			rmDir(".csg/meta/" + contenthash)
			delete(me.meta.ContentHashes, filename)
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
