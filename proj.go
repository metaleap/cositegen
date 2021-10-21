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
	Desc   map[string]string
	Series []*Series
	Books  []*Book
	Langs  []string
	Qualis []struct {
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
			IdsToFileNames map[string]string
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

func (me *Project) seriesByName(name string) *Series {
	for _, series := range me.Series {
		if series.Name == name {
			return series
		}
	}
	return nil
}

func (me *Project) percentTranslated(lang string, ser *Series, chap *Chapter, sheetVer *SheetVer, pgNr int) float64 {
	numtotal, numtrans := 0, 0
	for _, series := range me.Series {
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
	if fileStat(".ccache/data.json") != nil {
		jsonLoad(".ccache/data.json", nil, &me.data)
	}
	if fileStat("texts.json") != nil {
		jsonLoad("texts.json", nil, &me.data.Sv.textRects)
	} else {
		me.data.Sv.textRects = map[string][][]ImgPanelArea{}
	}
	me.data.Sv.fileNamesToIds, me.data.Sv.IdsToFileNames = map[string]string{}, map[string]string{}
	if me.data.Sv.ById == nil {
		me.data.Sv.ById = map[string]*SheetVerData{}
	}
	if me.data.PngOpt == nil {
		me.data.PngOpt = map[string][]string{}
	}

	for _, book := range me.Books {
		book.parentProj = me
		if len(book.Title) == 0 {
			book.Title = map[string]string{me.Langs[0]: book.Name}
		}
	}
	for _, series := range me.Series {
		series.parentProj = me
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
					go func(sv *SheetVer) {
						data := fileRead(sv.fileName)
						sv.id = contentHashStr(data)
						work.Lock()
						me.data.Sv.fileNamesToIds[sv.fileName] = sv.id
						me.data.Sv.IdsToFileNames[sv.id] = sv.fileName
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
					}(sheetver)
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
		if me.data.Sv.IdsToFileNames[svid] == "" {
			delete(me.data.Sv.ById, svid)
			rmDir(".ccache/" + svid)
		}
	}
	return
}
