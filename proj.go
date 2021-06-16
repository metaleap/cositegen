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
	Langs  []struct {
		Name  string
		Title string
	}
	Qualis []struct {
		Name     string
		SizeHint int
	}
	MaxImagePanelTextAreas int
	PageContentTexts       map[string]map[string]string
	Gen                    struct {
		IdQualiList  string
		ClsSeries    string
		ClsChapter   string
		ClsPanelCols string
		ClsPanelCol  string
		ClsPanelRows string
		ClsPanelRow  string
		ClsPanel     string
		ClsPanelArea string
		APaging      string
		PanelSvgText struct {
			PerLineDy     string
			Css           map[string][]string
			AppendToFiles map[string]bool
		}
	}

	meta struct {
		ContentHashes map[string]string
		SheetVer      map[string]*SheetVerMeta
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

	dirPath string
	sheets  []*Sheet
}

func (me *Chapter) At(i int) fmt.Stringer { return me.sheets[i] }
func (me *Chapter) Len() int              { return len(me.sheets) }
func (me *Chapter) String() string        { return me.Name }

func (me *Project) save() {
	jsonSave(".csg_meta.json", &me.meta)
}

func (me *Project) load() {
	jsonLoad("cosite.json", me)
	if _, err := os.Stat(".csg_meta.json"); err == nil {
		jsonLoad(".csg_meta.json", &me.meta)
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	if me.meta.ContentHashes == nil {
		me.meta.ContentHashes = map[string]string{}
	}
	if me.meta.SheetVer == nil {
		me.meta.SheetVer = map[string]*SheetVerMeta{}
	}

	for filename, contenthash := range me.meta.ContentHashes {
		if fileinfo, err := os.Stat(filename); err != nil || fileinfo.IsDir() {
			delete(me.meta.SheetVer, contenthash)
			_ = os.RemoveAll(filepath.Join(".csg_meta", contenthash))
			delete(me.meta.ContentHashes, filename)
		}
	}
	for _, series := range me.Series {
		series.dirPath = series.Name
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
					versionname := fnamebase[strings.LastIndexByte(fnamebase, '-')+1:]
					if versionname == fnamebase {
						panic("invalid sheet-file name: " + fname)
					}
					sheetname := fnamebase[:strings.LastIndexByte(fnamebase, '-')]
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
					App.PrepWork.Queue = append(App.PrepWork.Queue, sheetver)
				}
			}
		}
	}
}
