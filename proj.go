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
	Desc        map[string]string
	Series      []*Series
	BookDefs    map[string]*BookDef
	BookConfigs map[string]*BookConfig
	BookBuilds  map[string]*BookBuild
	Langs       []string
	Qualis      []struct {
		Name     string
		SizeHint int
	}
	AtomFile struct {
		PubDates    []string
		Name        string
		ContentHtml map[string]string
	}
	MaxImagePanelTextAreas int
	BwThresholds           []uint8
	BwSmallWidth           uint16
	PanelBorderCm          float64
	PanelBgFileExt         string
	PanelBgScaleIfPng      float64
	PanelBgBlur            int
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
			TspanTagClasses      []string
		}
	}

	allPrepsDone bool
	data         struct {
		Sv struct {
			fileNamesToIds map[string]string
			IdsToFileMeta  map[string]FileInfo
			ById           map[string]*SheetVerData

			textRects map[string][][]ImgPanelArea
		}
		PngOpt map[string][]string
	}
}

func (me *Project) At(i int) fmt.Stringer { return me.Series[i] }
func (me *Project) Len() int              { return len(me.Series) }

func (me *Project) hasSvgQuali() bool {
	for _, q := range me.Qualis {
		if q.SizeHint == 0 {
			return true
		}
	}
	return false
}

func (me *Project) maxQualiIdx() (r int) {
	for i, quali := range me.Qualis {
		if quali.SizeHint > me.Qualis[r].SizeHint {
			r = i
		}
	}
	return
}

func (me *Project) seriesByName(name string) *Series {
	for _, series := range me.Series {
		if series.Name == name {
			return series
		}
	}
	return nil
}

func (me *Project) textStr(lang string, key string) (s string) {
	if s = me.PageContentTexts[lang][key]; s == "" {
		if s = me.PageContentTexts[me.Langs[0]][key]; s == "" {
			s = key
		}
	}
	return s
}

func (me *Project) percentTranslated(lang string, ser *Series, chap *Chapter, sheetVer *SheetVer, pgNr int) float64 {
	numtotal, numtrans, allseries := 0, 0, me.Series
	if ser != nil && ser.Book != nil {
		allseries = []*Series{ser}
	}
	for _, series := range allseries {
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
					for _, areas := range me.data.Sv.textRects[sv.id] {
						for _, area := range areas {
							if def := trim(area.Data[me.Langs[0]]); def != "" {
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

func (me *Project) save() {
	jsonSave(".ccache/data.json", &me.data)
	jsonSave("texts.json", me.data.Sv.textRects)
}

func (me *Project) load() (numSheetVers int) {
	jsonLoad("comicsite.json", nil, me) // exits early if no such file, before creating work dirs:
	mkDir(".ccache")
	var dtdatajson time.Time
	if fileinfo := fileStat(".ccache/data.json"); fileinfo != nil {
		dtdatajson = fileinfo.ModTime()
		jsonLoad(".ccache/data.json", nil, &me.data)
	}
	if fileStat("texts.json") != nil {
		jsonLoad("texts.json", nil, &me.data.Sv.textRects)
	} else {
		me.data.Sv.textRects = map[string][][]ImgPanelArea{}
	}
	me.data.Sv.fileNamesToIds = map[string]string{}
	if me.data.Sv.IdsToFileMeta == nil {
		me.data.Sv.IdsToFileMeta = map[string]FileInfo{}
	}
	if me.data.Sv.ById == nil {
		me.data.Sv.ById = map[string]*SheetVerData{}
	}
	if me.data.PngOpt == nil {
		me.data.PngOpt = map[string][]string{}
	}

	if me.BookDefs == nil {
		me.BookDefs = map[string]*BookDef{}
	}
	if me.BookBuilds == nil {
		me.BookBuilds = map[string]*BookBuild{}
	}
	if me.BookConfigs == nil {
		me.BookConfigs = map[string]*BookConfig{}
	}
	for name, bookdef := range me.BookDefs {
		bookdef.name = name
		if len(bookdef.Title) == 0 {
			bookdef.Title = map[string]string{me.Langs[0]: name}
		}
	}
	for name, bb := range me.BookBuilds {
		bb.name = name
		bb.config = *me.BookConfigs[bb.Config]
		bb.book = *me.BookDefs[bb.Book]
	}
	for _, series := range me.Series {
		seriesdirpath := "scans/" + series.Name
		if series.UrlName == "" {
			series.UrlName = series.Name
		}
		if len(series.Title) == 0 {
			series.Title = map[string]string{me.Langs[0]: series.Name}
		}
		for _, chap := range series.Chapters {
			chap.parentSeries = series
			chapdirpath := filepath.Join(seriesdirpath, chap.Name)
			if chap.UrlName == "" {
				chap.UrlName = chap.Name
			}
			if len(chap.Title) == 0 {
				chap.Title = map[string]string{me.Langs[0]: chap.Name}
			}
			files, err := os.ReadDir(chapdirpath)
			if err != nil {
				panic(err)
			}

			var work = struct {
				sync.WaitGroup
				sync.Mutex
			}{}
			for _, f := range files {
				if fnamebase := f.Name(); strings.HasSuffix(fnamebase, ".png") && !f.IsDir() {
					fname := filepath.Join(chapdirpath, fnamebase)
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
					fileinfo, err := f.Info()
					if err != nil {
						panic(err)
					}
					go func(sv *SheetVer, svfileinfo os.FileInfo) {
						if modtime := svfileinfo.ModTime().UnixNano(); modtime < dtdatajson.UnixNano() {
							work.Lock()
							for id, filemeta := range me.data.Sv.IdsToFileMeta {
								if filemeta.FilePath == sv.fileName && filemeta.ModTime == modtime && filemeta.Size == svfileinfo.Size() {
									sv.id = id
									break
								}
							}
							work.Unlock()
						}
						if sv.id == "" {
							data := fileRead(sv.fileName)
							sv.id = contentHashStr(data)
						}
						work.Lock()
						me.data.Sv.fileNamesToIds[sv.fileName] = sv.id
						me.data.Sv.IdsToFileMeta[sv.id] = FileInfo{sv.fileName, svfileinfo.ModTime().UnixNano(), svfileinfo.Size()}
						work.Unlock()
						if sv.data = me.data.Sv.ById[sv.id]; sv.data != nil {
							sv.data.parentSheetVer = sv
						}
						cachedirsymlinkpath := sv.fileName[:len(sv.fileName)-len(".png")]
						_ = os.Remove(cachedirsymlinkpath)
						if err := os.Symlink("../../../.ccache/"+sv.id, cachedirsymlinkpath); err != nil {
							panic(err)
						}
						work.Done()
					}(sheetver, fileinfo)
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
		if _, exists := me.data.Sv.IdsToFileMeta[svid]; !exists {
			delete(me.data.Sv.ById, svid)
			rmDir(".ccache/" + svid)
		}
	}
	return
}
