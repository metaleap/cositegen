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
	Name         string
	Title        string
	ScansPerPage int

	dirPath string
	scans   []*Scan
}

func (me *Chapter) At(i int) fmt.Stringer { return me.scans[i] }
func (me *Chapter) Len() int              { return len(me.scans) }
func (me *Chapter) String() string        { return me.Name }

type Scan struct {
	name     string
	versions []*ScanVersion
}

func (me *Scan) At(i int) fmt.Stringer { return me.versions[i] }
func (me *Scan) Len() int              { return len(me.versions) }
func (me *Scan) String() string        { return me.name }

type ScanVersion struct {
	parent   *Scan
	name     string
	fileName string
}

func (me *ScanVersion) String() string { return me.name }

func (me *Project) Load(filename string) {
	jsonLoad(filename, &App.Proj)
	for _, series := range me.Series {
		series.dirPath = series.Name
		for _, chapter := range series.Chapters {
			chapter.dirPath = filepath.Join(series.dirPath, chapter.Name)
			files, err := ioutil.ReadDir(filepath.Join(chapter.dirPath, "scans"))
			if err != nil {
				panic(err)
			}
			for _, f := range files {
				if fnamebase := f.Name(); !f.IsDir() {
					fname := filepath.Join(chapter.dirPath, fnamebase)
					fnamebase = fnamebase[:len(fnamebase)-len(filepath.Ext(fnamebase))]
					versionname := fnamebase[strings.LastIndexByte(fnamebase, '-')+1:]
					if versionname == fnamebase {
						panic("invalid scan-file name: " + fname)
					}
					scanname := fnamebase[:strings.LastIndexByte(fnamebase, '-')]
					if scanname == "" {
						panic("invalid scan-file name: " + fname)
					}

					var scan *Scan
					for _, s := range chapter.scans {
						if s.name == scanname {
							scan = s
							break
						}
					}
					if scan == nil {
						scan = &Scan{name: scanname}
						println("addscan", scan.name)
						chapter.scans = append(chapter.scans, scan)
					}

					var scanver *ScanVersion
					for _, sv := range scan.versions {
						if sv.name == versionname {
							scanver = sv
							break
						}
					}
					if scanver == nil {
						scanver = &ScanVersion{name: versionname, parent: scan, fileName: fname}
						scan.versions = append(scan.versions, scanver)
						println("addver", versionname, "to", scan.name, "filename", fname)
					}
				}
			}
		}
	}
}
