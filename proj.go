package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type Indexed interface {
	At(int) fmt.Stringer
	Len() int
}

type Project struct {
	Title  string
	Desc   string
	Series []*Series

	meta struct {
		ContentHashes map[string]string
	}
}

func (me *Project) At(i int) fmt.Stringer { return me.Series[i] }
func (me *Project) Len() int              { return len(me.Series) }

type Series struct {
	Name     string
	Title    string
	Chapters []*Chapter

	dirPath string
}

func (me *Series) At(i int) fmt.Stringer { return me.Chapters[i] }
func (me *Series) Len() int              { return len(me.Chapters) }
func (me *Series) String() string        { return me.Name }

type Chapter struct {
	Name          string
	Title         string
	SheetsPerPage int

	dirPath string
	sheets  []*Sheet
}

func (me *Chapter) At(i int) fmt.Stringer { return me.sheets[i] }
func (me *Chapter) Len() int              { return len(me.sheets) }
func (me *Chapter) String() string        { return me.Name }

func (me *Project) load(filename string) {
	jsonLoad("."+filename, &me.meta, []byte("{}"))
	if me.meta.ContentHashes == nil {
		me.meta.ContentHashes = map[string]string{}
	}

	jsonLoad(filename, me, nil)
	for _, series := range me.Series {
		series.dirPath = series.Name
		for _, chapter := range series.Chapters {
			chapter.dirPath = filepath.Join(series.dirPath, chapter.Name)
			sheetsdirpath := filepath.Join(chapter.dirPath, "sheets")
			files, err := ioutil.ReadDir(sheetsdirpath)
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
						if sv.name == versionname {
							panic("ASSERT")
						}
					}
					sheetver := &SheetVersion{name: versionname, parent: sheet, fileName: fname}
					sheet.versions = append(sheet.versions, sheetver)
				}
			}
		}
	}
}
