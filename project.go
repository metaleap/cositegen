package main

import (
	"io/ioutil"
	"path/filepath"
	"strings"
)

type Project struct {
	Title  string
	Desc   string
	Series []*Series
}

type Series struct {
	Name     string
	Title    string
	Chapters []*Chapter

	dirPath string
}

type Chapter struct {
	Name         string
	Title        string
	ScansPerPage int

	dirPath string
	scans   []*Scan
}

type Scan struct {
	name     string
	versions []*ScanVersion
}

type ScanVersion struct {
	parent   *Scan
	name     string
	fileName string
}

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
