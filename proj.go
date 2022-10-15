package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DirMode struct {
	Name  string
	Title map[string]string
}

type Project struct {
	Langs  []string
	Qualis []struct {
		Name               string
		SizeHint           int
		ExcludeInSiteGen   bool
		HomeAndFeedDefault bool
	}
	DirModes struct {
		Ltr DirMode
		Rtl DirMode
	}
	Authors map[string]*Author
	Series  []*Series
	Books   struct {
		RepoPath struct {
			Prefix string
			Infix  string
		}
		Pubs []struct {
			Title           string
			RepoName        string
			Year            int
			Series          []string
			PubDate         string
			LibGenEditionID string
		}
	}
	Site struct {
		Title string
		Host  string
		Desc  map[string]string
		Feed  struct {
			Name           string
			PubDates       []string
			ContentTxt     map[string]string
			ContentTxtBook map[string]string
		}
		Texts map[string]map[string]string
		Gen   struct {
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
			APaging          string
			ImgSrcLang       string
			PicDirName       string
		}
	}
	Sheets struct {
		Bw struct {
			SmallWidth uint16
			Thresholds struct {
				Previewable []uint8
				Defaults    map[string]uint8
			}
			NumDistrClusters int
		}
		Panel struct {
			MaxNumTextAreas int
			BorderCm        float64
			BgScale         float64
			BgBlur          int
			CssFontFaces    map[string]string
			SvgText         map[string]*PanelSvgTextGen
		}
	}

	defaultQualiIdx int
	allPrepsDone    bool
	data            struct {
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

func (me *Project) maxQualiIdx(siteGenOnly bool) (r int) {
	for i, quali := range me.Qualis {
		if quali.SizeHint > me.Qualis[r].SizeHint && ((!siteGenOnly) || !quali.ExcludeInSiteGen) {
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

func (me *Project) dirMode(rtl bool) *DirMode {
	if rtl {
		return &me.DirModes.Rtl
	}
	return &me.DirModes.Ltr
}

func (me *Project) textStr(lang string, key string) (s string) {
	if s = me.Site.Texts[lang][key]; s == "" {
		if s = me.Site.Texts[me.Langs[0]][key]; s == "" {
			s = key
		}
	}
	return s
}

func (me *Project) percentTranslated(lang string, ser *Series, chap *Chapter, sheetVer *SheetVer, pgNr int) float64 {
	numtotal, numtrans, allseries := 0, 0, me.Series
	if ser != nil {
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
				if pgNr > 0 && !chapter.isSheetOnPgNr(pgNr, i) {
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

func (me *Project) save(texts bool) {
	if jsonSave("_data.json", &me.data); texts {
		jsonSave("_txt.json", me.data.Sv.textRects)
	}
}

func (me *Project) load() (numSheetVers int) {
	jsonLoad("cx.json", nil, me) // exits early if no such file, before creating work dirs:
	if me.Site.Title == "" {
		me.Site.Title = me.Site.Host
	}
	mkDir(".ccache")
	var dtdatajson time.Time
	if fileinfo := fileStat("_data.json"); fileinfo != nil {
		dtdatajson = fileinfo.ModTime()
		jsonLoad("_data.json", nil, &me.data)
	}
	if fileStat("_txt.json") != nil {
		jsonLoad("_txt.json", nil, &me.data.Sv.textRects)
	} else {
		me.data.Sv.textRects = map[string][][]ImgPanelArea{}
	}
	me.data.Sv.fileNamesToIds = map[string]string{}
	oldIdsToFileMeta := me.data.Sv.IdsToFileMeta
	me.data.Sv.IdsToFileMeta = make(map[string]FileInfo, len(oldIdsToFileMeta))
	if me.data.Sv.ById == nil {
		me.data.Sv.ById = map[string]*SheetVerData{}
	}
	if me.data.PngOpt == nil {
		me.data.PngOpt = map[string][]string{}
	}
	for i, q := range me.Qualis {
		if q.Name = trim(q.Name); q.Name == "" {
			q.Name = itoa(q.SizeHint)
		}
		if me.Qualis[i] = q; q.HomeAndFeedDefault {
			me.defaultQualiIdx = i
		}
	}

	if me.Authors == nil {
		me.Authors = map[string]*Author{}
	}

	svgtxt := me.Sheets.Panel.SvgText[""]
	for k, v := range me.Sheets.Panel.CssFontFaces {
		if svgtxt.TspanSubTagStyles[k] == "" {
			svgtxt.TspanSubTagStyles[k] = v[:1+strings.IndexByte(v, ';')] + " !important;"
		}
	}
	me.Sheets.Panel.SvgText[""] = svgtxt
	for k, v := range me.Sheets.Panel.SvgText {
		if k != "" {
			v.baseOn(nil)
		}
	}

	for _, series := range me.Series {
		if series.GenPanelSvgText == nil {
			series.GenPanelSvgText = me.Sheets.Panel.SvgText[""]
		} else {
			series.GenPanelSvgText.baseOn(nil)
			series.GenPanelSvgText.ser, me.Sheets.Panel.SvgText[series.Name] = true, series.GenPanelSvgText
		}
		if series.Author != "" {
			if series.author = me.Authors[series.Author]; series.author == nil {
				panic("unknown author: " + series.Author)
			}
		}
		seriesdirpath := "scans/" + series.Name
		if series.UrlName == "" {
			series.UrlName = series.Name
		}
		if len(series.Title) == 0 {
			series.Title = map[string]string{me.Langs[0]: series.Name}
		}
		for _, chap := range series.Chapters {
			if chap.GenPanelSvgText == nil {
				chap.GenPanelSvgText = series.GenPanelSvgText
			} else {
				chap.GenPanelSvgText.baseOn(series.GenPanelSvgText)
				chap.GenPanelSvgText.chap, me.Sheets.Panel.SvgText[chap.Name] = true, chap.GenPanelSvgText
			}
			if chap.Year == 0 {
				chap.Year = series.Year
			}
			if chap.StoryUrls.LinkHref == "" {
				chap.StoryUrls = series.StoryUrls
			}
			if chap.StoryUrls.DisplayUrl == "" && chap.StoryUrls.LinkHref != "" {
				chap.StoryUrls.DisplayUrl = chap.StoryUrls.LinkHref
				if urldatestr := "2021"; !strings.HasPrefix(chap.StoryUrls.LinkHref, "archive.") {
					if len(chap.Name) > 4 && chap.Name[4] == '-' {
						if _, err := strconv.Atoi(chap.Name[:4]); err == nil {
							urldatestr = "20" + chap.Name[:4]
						}
					}
					chap.StoryUrls.LinkHref = "web.archive.org/web/" + urldatestr + "/" + chap.StoryUrls.LinkHref
				}
			}

			if chap.Author == "" {
				chap.author = series.author
			} else if chap.author = me.Authors[chap.Author]; chap.author == nil {
				panic("unknown author: " + chap.Author)
			}

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
				if fnamebase := f.Name(); strings.HasSuffix(fnamebase, ".png") &&
					!(f.IsDir() || strings.HasPrefix(fnamebase, "bw.") || strings.HasPrefix(fnamebase, "p.")) {
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

					fileinfo, err := f.Info()
					if err != nil {
						panic(err)
					}
					work.Add(1)
					go func(sv *SheetVer, svfileinfo os.FileInfo) {
						defer work.Done()
						if modtime := svfileinfo.ModTime().UnixNano(); modtime < dtdatajson.UnixNano() {
							work.Lock()
							for id, filemeta := range oldIdsToFileMeta {
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
						if err := os.Symlink("../../../.ccache/"+svCacheDirNamePrefix+sv.id, cachedirsymlinkpath); err != nil {
							panic(err)
						}
					}(sheetver, fileinfo)
				}
			}
			work.Wait()

			if len(chap.sheets) > 0 {
				chap.ensureSheetsPerPage()
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
			rmDir(".ccache/" + svCacheDirNamePrefix + svid)
		}
	}
	{ // TODO: detect and remove old stale no-longer-needed svdata dirs in .ccache
		entries, err := os.ReadDir(".ccache")
		if err != nil {
			panic(err)
		}
		for _, entry := range entries {
			if name := entry.Name(); entry.IsDir() && svCacheDirNamePrefix != "" && strings.HasPrefix(name, svCacheDirNamePrefix) {
				printLn(len(name), me.data.Sv.ById[name] == nil, name, me.data.Sv.IdsToFileMeta[name])
			}
		}
	}

	return
}

func (me *Project) numSheets(skipPriv bool, lang string) (ret int) {
	for _, series := range me.Series {
		if series.Priv && skipPriv {
			continue
		}
		ret += series.numSheets(skipPriv, lang)
	}
	return
}

func (me *Project) numPanels(skipPriv bool, lang string) (ret int) {
	for _, series := range me.Series {
		if series.Priv && skipPriv {
			continue
		}
		ret += series.numPanels(skipPriv, lang)
	}
	return
}

func (me *Project) numPages(skipPriv bool, lang string) (ret int) {
	for _, series := range me.Series {
		if series.Priv && skipPriv {
			continue
		}
		ret += series.numPages(skipPriv, lang)
	}
	return
}

func (me *Project) scanYearLatest(skipPriv bool, lang string) (ret int) {
	for _, series := range me.Series {
		if series.Priv && skipPriv {
			continue
		}
		if year := series.scanYearLatest(skipPriv, lang); year > ret {
			ret = year
		}
	}
	return
}

func (me *Project) bwThreshold(dtUnixNano int64) (ret uint8) {
	if ret = me.Sheets.Bw.Thresholds.Defaults[""]; dtUnixNano > 0 {
		var d int64
		for k, v := range me.Sheets.Bw.Thresholds.Defaults {
			if dt, _ := strconv.ParseInt(k, 0, 64); dt != 0 {
				if diff := dtUnixNano - dt; diff >= 0 && diff <= d {
					ret, d = v, diff
				}
			}
		}
	}
	return
}

func (me *Project) cssFontFaces(repl *strings.Replacer) (css string) {
	for _, k := range sortedMapKeys(me.Sheets.Panel.CssFontFaces) {
		v := me.Sheets.Panel.CssFontFaces[k]
		if repl != nil {
			v = repl.Replace(v)
		}
		css = "@font-face { " + v + "}\n." + k + "{" + v[:1+strings.IndexByte(v, ';')] + "}\n" + css
	}
	return
}
