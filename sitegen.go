package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"
)

var (
	SiteTitleOrig = os.Getenv(strIf(os.Getenv("NOLINKS") == "", "CSG", "CSG_"))
	SiteTitleEsc  = strings.NewReplacer("<span>", "", "</span>", "", "&bull;", ".").Replace(SiteTitleOrig)
	viewModes     = []string{"s", "r"}
)

type PageGen struct {
	SiteTitleOrig    string
	SiteTitleOrigSub string
	SiteTitleEsc     string
	SiteDesc         string
	PageTitle        string
	PageTitleTxt     string
	PageDesc         string
	PageDescTxt      string
	PageLang         string
	PageCssClasses   string
	PageDirCur       string
	PageDirAlt       string
	DirCurTitle      string
	DirAltTitle      string
	DirCurDesc       string
	DirAltDesc       string
	LangsList        string
	ViewerList       string
	HrefViewAlt      string
	HrefViewCur      string
	QualList         string
	PagesList        string
	PageContent      string
	HintHtmlR        string
	HintHtmlS        string
	HintDir          string
	VerHint          string
	LegalHtml        string
	HrefHome         string
	HrefDirLtr       string
	HrefDirRtl       string
	HrefDirCur       string
	HrefDirAlt       string
	HrefFeed         string
	VersList         string
	ColsList         string
	ChapTitle        string
}

type siteGen struct {
	tmpl       *template.Template
	page       PageGen
	lang       string
	bgCol      bool
	dirRtl     bool
	onPicSize  func(*Chapter, string, int, int64)
	maxPicSize uint32
	sheetPgNrs map[*SheetVer]int
}

func (me siteGen) genSite(fromGui bool, _ map[string]bool) {
	var err error
	tstart := time.Now()
	me.sheetPgNrs = map[*SheetVer]int{}
	printLn("SiteGen started. When done, result will open in new window.")
	if fromGui {
		defer func() {
			if err := recover(); err != nil {
				printLn("SiteGen Error: ", err)
			}
		}()
	}
	rmDir(".build")
	mkDir(".build")
	mkDir(".build/" + App.Proj.Gen.PicDirName)

	timedLogged("SiteGen: copying static files to .build...", func() string {
		if App.Proj.Gen.PanelSvgText.AppendToFiles == nil {
			App.Proj.Gen.PanelSvgText.AppendToFiles = map[string]bool{}
		}
		numfilescopied := me.copyStaticFiles("")
		return "for " + strconv.Itoa(numfilescopied) + " files"
	})

	timedLogged("SiteGen: generating (but mostly copying pre-generated) PNGs & SVGs...", func() string {
		chapterqstats := map[*Chapter]map[string][]int64{}
		for _, series := range App.Proj.Series {
			for _, chapter := range series.Chapters {
				chapterqstats[chapter] = map[string][]int64{}
			}
		}
		var mu sync.Mutex
		me.onPicSize = func(chapter *Chapter, id string, qidx int, size int64) {
			mu.Lock()
			m := chapterqstats[chapter]
			sl := m[id]
			if len(sl) == 0 {
				sl = make([]int64, len(App.Proj.Qualis))
			}
			sl[qidx] += size
			m[id] = sl
			mu.Unlock()
		}
		numsvgs, numpngs, numsheets, numpanels, totalsize := me.genOrCopyPanelPics()
		numpngs += me.copyHomeThumbsPngs()
		qstats := make(map[*Chapter][]int64, len(chapterqstats))
		for chapter, namesandsizes := range chapterqstats {
			min, chq := int64(0), make([]int64, len(App.Proj.Qualis))
			for _, sizes := range namesandsizes {
				for qidx, size := range sizes {
					for i := qidx; size == 0; i-- {
						size = sizes[i]
					}
					chq[qidx] += size
				}
			}
			qstats[chapter] = chq
			for qidx, totalsize := range chq {
				if (min == 0 || totalsize < min) && App.Proj.Qualis[qidx].SizeHint <= 4096 && App.Proj.Qualis[qidx].SizeHint > 0 {
					min, chapter.defaultQuali = totalsize, qidx
				}
				pref := chapter.Name
				if qidx > 0 {
					pref = strings.Repeat(" ", len(pref))
				}
				printLn("\t\t" + pref + "\t\t" + App.Proj.Qualis[qidx].Name + "(" + itoa(App.Proj.Qualis[qidx].SizeHint) + ") => " + strSize64(totalsize))
			}
		}
		return "for " + itoa(int(numpngs)) + " PNGs & " + itoa(int(numsvgs)) + " SVGs (" + strSize(int(totalsize)) + ") from " + itoa(int(numpanels)) + " panels in " + itoa(int(numsheets)) + " sheets, max panel pic size: " + strSize(int(me.maxPicSize))
	})

	timedLogged("SiteGen: generating markup files...", func() string {
		numfileswritten := 0
		me.tmpl, err = template.New("foo").ParseFiles(siteTmplDirName + "/site.html")
		if err != nil {
			panic(err)
		}
		for _, me.lang = range App.Proj.Langs {
			for _, me.dirRtl = range []bool{true, false /*KEEP this order of bools*/} {
				me.bgCol = false
				numfileswritten += me.genPages(nil, 0)
				for _, me.bgCol = range []bool{false, true} {
					for _, series := range App.Proj.Series {
						for _, chapter := range series.Chapters {
							if me.bgCol && !chapter.HasBgCol() {
								continue
							}
							if chapter.SheetsPerPage > 0 {
								for i := 1; i <= (len(chapter.sheets) / chapter.SheetsPerPage); i++ {
									numfileswritten += me.genPages(chapter, i)
								}
							} else {
								numfileswritten += me.genPages(chapter, 0)
							}
						}
					}
				}
				if App.Proj.AtomFile.Name != "" {
					numfileswritten += me.genAtomXml()
				}
			}
		}
		return "for " + strconv.Itoa(numfileswritten) + " files"
	})

	printLn("SiteGen: DONE after " + time.Now().Sub(tstart).String())
	cmd := exec.Command(browserCmd[0], append(browserCmd[1:], "--app=file://"+os.Getenv("PWD")+"/.build/index.html")...)
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}

func (me *siteGen) copyStaticFiles(relDirPath string) (numFilesWritten int) {
	srcdirpath := filepath.Join(siteTmplDirName, relDirPath)
	if fileinfos, err := os.ReadDir(srcdirpath); err != nil {
		panic(err)
	} else {
		for _, fileinfo := range fileinfos {
			fn := fileinfo.Name()
			relpath := filepath.Join(relDirPath, fn)
			dstpath := filepath.Join(".build", relpath)
			if fileinfo.IsDir() {
				mkDir(dstpath)
				numFilesWritten += me.copyStaticFiles(relpath)
			} else if fn != siteTmplFileName {
				data := fileRead(filepath.Join(srcdirpath, fn))
				if App.Proj.Gen.PanelSvgText.AppendToFiles[relpath] {
					for csssel, csslines := range App.Proj.Gen.PanelSvgText.Css {
						if csssel != "" && csssel != "@font-face" {
							if csslines == nil {
								csslines = App.Proj.Gen.PanelSvgText.Css[""]
							}
							data = append([]byte(csssel+"{"+strings.Join(csslines, ";")+"}\n"), data...)
						}
					}
					if cssff := App.Proj.Gen.PanelSvgText.Css["@font-face"]; len(cssff) != 0 {
						data = append([]byte("@font-face{"+strings.Join(cssff, ";")+"}\n"), data...)
					}
				}
				fileWrite(dstpath, data)
				numFilesWritten++
			}
		}
	}
	return
}

func (me *siteGen) genOrCopyPanelPics() (numSvgs uint32, numPngs uint32, numSheets uint32, numPanels uint32, totalSize uint64) {
	var work sync.WaitGroup
	atomic.StoreUint32(&numSheets, 0)
	atomic.StoreUint32(&numPngs, 0)
	atomic.StoreUint32(&numSvgs, 0)
	atomic.StoreUint32(&numPanels, 0)
	atomic.StoreUint64(&totalSize, 0)
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			for _, sheet := range chapter.sheets {
				work.Add(1)
				go func(chapter *Chapter, sheet *Sheet) {
					for _, sv := range sheet.versions {
						nsvgs, npngs, npnls, totalsize := me.genOrCopyPanelPicsOf(sv)
						atomic.AddUint32(&numSheets, 1)
						atomic.AddUint32(&numPngs, npngs)
						atomic.AddUint32(&numPanels, npnls)
						atomic.AddUint32(&numSvgs, nsvgs)
						atomic.AddUint64(&totalSize, totalsize)
					}
					work.Done()
				}(chapter, sheet)
			}
		}
	}
	work.Wait()
	return
}

func (me *siteGen) genOrCopyPanelPicsOf(sv *SheetVer) (numSvgs uint32, numPngs uint32, numPanels uint32, totalSize uint64) {
	sv.ensurePrep(false, false)
	atomic.StoreUint32(&numPngs, 0)
	atomic.StoreUint32(&numSvgs, 0)
	atomic.StoreUint64(&totalSize, 0)
	var pidx int
	var work sync.WaitGroup
	sv.data.PanelsTree.iter(func(panel *ImgPanel) {
		work.Add(1)
		numPanels++
		go func(pidx int) {
			for qidx, quali := range App.Proj.Qualis {
				if quali.Name == "" {
					continue
				}
				fext := strIf(quali.SizeHint == 0, ".svg", ".png")
				srcpath := filepath.Join(sv.data.PicDirPath(quali.SizeHint), itoa(pidx)+fext)
				if fileinfo := fileStat(srcpath); fileinfo == nil && quali.SizeHint != 0 {
					break
				} else {
					for fs, swap := uint32(fileinfo.Size()), true; swap; {
						max := atomic.LoadUint32(&me.maxPicSize)
						swap = fs > max && !atomic.CompareAndSwapUint32(&me.maxPicSize, max, fs)
					}
					atomic.AddUint64(&totalSize, uint64(fileinfo.Size()))
					dstpath := filepath.Join(".build/"+App.Proj.Gen.PicDirName+"/", me.namePanelPic(sv, pidx, quali.SizeHint)+fext)
					fileLinkOrCopy(srcpath, dstpath)
					if me.onPicSize != nil {
						me.onPicSize(sv.parentSheet.parentChapter, sv.id+itoa(pidx), qidx, fileinfo.Size())
					}
					if quali.SizeHint == 0 {
						atomic.AddUint32(&numSvgs, 1)
					} else {
						atomic.AddUint32(&numPngs, 1)
					}
				}
			}
			if srcpath := filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+App.Proj.PanelBgFileExt); sv.data.hasBgCol {
				if fileinfo := fileStat(srcpath); fileinfo != nil {
					atomic.AddUint64(&totalSize, uint64(fileinfo.Size()))
					dstpath := filepath.Join(".build/" + App.Proj.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg" + App.Proj.PanelBgFileExt)
					fileLinkOrCopy(srcpath, dstpath)
					if App.Proj.PanelBgFileExt == ".png" {
						atomic.AddUint32(&numPngs, 1)
					} else {
						atomic.AddUint32(&numSvgs, 1)
					}
				}
			}
			work.Done()
		}(pidx)
		pidx++
	})

	work.Wait()
	return
}

func (me *siteGen) genPages(chapter *Chapter, pageNr int) (numFilesWritten int) {
	strrepl := locStr(App.Proj.DirModes.Ltr.Title, me.lang)
	if me.dirRtl {
		strrepl = locStr(App.Proj.DirModes.Rtl.Title, me.lang)
	}
	homename, repl := "index", strings.NewReplacer(
		"%DIR%", "<span class='"+App.Proj.DirModes.Ltr.Name+App.Proj.DirModes.Rtl.Name+"'>"+strrepl+"</span>",
		"%LANG"+me.lang+"%", itoa(int(App.Proj.percentTranslated(me.lang, nil, nil, nil, -1))),
	)
	me.page = PageGen{
		SiteTitleEsc:     SiteTitleEsc,
		SiteTitleOrig:    SiteTitleOrig,
		SiteTitleOrigSub: strings.TrimPrefix(strings.TrimSuffix(SiteTitleOrig[5:], "</span>"), "<span>"),
		SiteDesc:         repl.Replace(hEsc(locStr(App.Proj.Desc, me.lang))),
		PageLang:         me.lang,
		HintHtmlR:        me.textStr("HintHtmlR"),
		HintHtmlS:        me.textStr("HintHtmlS"),
		HintDir:          me.textStr("HintDir"),
		VerHint:          me.textStr("VerHint"),
		LegalHtml:        me.textStr("LegalHtml"),
		HrefFeed:         "./" + App.Proj.AtomFile.Name + "." + me.lang + ".atom",
		PageDirCur:       "ltr",
		PageDirAlt:       "rtl",
	}
	if idx := strings.IndexByte(me.page.SiteDesc, '.'); idx > 0 {
		me.page.SiteDesc = "<nobr>" + me.page.SiteDesc[:idx+1] + "</nobr>" + me.page.SiteDesc[idx+1:]
	}
	if me.dirRtl {
		me.page.PageDirCur, me.page.PageDirAlt = "rtl", "ltr"
		homename += "." + App.Proj.DirModes.Rtl.Name
	}
	if me.lang != App.Proj.Langs[0] {
		homename += "." + me.lang
	}
	me.page.HrefHome = "./" + homename + ".html"

	if chapter == nil {
		me.page.PageTitle = hEsc(me.textStr("HomeTitle"))
		me.page.PageDesc = repl.Replace(hEsc(me.textStr("HomeDesc")))
		me.page.PageDescTxt = "Index"
		me.page.PageCssClasses = App.Proj.Gen.ClsChapter + "n"
		if me.lang == App.Proj.Langs[0] {
			me.page.HrefDirLtr = "./index.html"
			me.page.HrefDirRtl = "./index." + App.Proj.DirModes.Rtl.Name + ".html"
		} else {
			me.page.HrefDirLtr = "./index." + me.lang + ".html"
			me.page.HrefDirRtl = "./index." + App.Proj.DirModes.Rtl.Name + "." + me.lang + ".html"
		}
		me.prepHomePage()
		numFilesWritten += me.genPageExecAndWrite(homename, nil)

	} else {
		series := chapter.parentSeries
		me.page.HrefHome += "#" + strings.ToLower(series.Name)
		me.page.PageTitle = "<span>" + hEsc(locStr(series.Title, me.lang)) + ":</span> " + hEsc(locStr(chapter.Title, me.lang))
		me.page.PageTitleTxt = hEsc(locStr(series.Title, me.lang)) + ": " + hEsc(locStr(chapter.Title, me.lang))
		author := strIf(chapter.Author == "", series.Author, chapter.Author)
		if author = strIf(author == "?", "", author); author != "" {
			author = strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", strIf(author == "?", me.textStr("Unknown"), author), 1)
		}
		desc := locStr(chapter.Desc, me.lang)
		me.page.PageDesc = hEsc(strIf(desc == "", locStr(series.Desc, me.lang), desc)) + author
		me.page.PageDescTxt = hEsc(strIf(desc == "", locStr(series.Desc, me.lang), desc))
		for qidx, quali := range App.Proj.Qualis {
			if quali.Name == "" {
				continue
			}
			for _, viewmode := range viewModes {
				me.page.PageCssClasses = App.Proj.Gen.ClsChapter + viewmode
				for _, svdt := range chapter.versions {
					qname, qsizes, allpanels := quali.Name, map[int]int64{}, me.prepSheetPage(qidx, viewmode, chapter, svdt, pageNr)
					me.page.QualList = ""
					for i, q := range App.Proj.Qualis {
						var totalimgsize int64
						for sv, maxpidx := range allpanels {
							for pidx := 0; pidx <= maxpidx; pidx++ {
								if bgfile := fileStat(".build/" + App.Proj.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg" + App.Proj.PanelBgFileExt); bgfile != nil && me.bgCol {
									totalimgsize += bgfile.Size()
								}
								name := me.namePanelPic(sv, pidx, q.SizeHint)
								if fileinfo := fileStat(strings.ToLower(".build/" + App.Proj.Gen.PicDirName + "/" + name + strIf(q.SizeHint == 0, ".svg", ".png"))); fileinfo != nil {
									totalimgsize += fileinfo.Size()
								}
							}
						}
						if totalimgsize != 0 {
							qsizes[i] = totalimgsize
						} else if q.Name == qname {
							qname = App.Proj.Qualis[len(qsizes)-1].Name
						}
					}
					for i, q := range App.Proj.Qualis[:len(qsizes)] {
						if q.Name == "" {
							continue
						}
						me.page.QualList += "<option value='" + me.namePage(chapter, q.SizeHint, pageNr, viewmode, "", me.lang, svdt, me.bgCol) + "'"
						if q.Name == qname {
							me.page.QualList += " selected='selected'"
						}
						me.page.QualList += ">" + q.Name + " (" + strSize64(qsizes[i]) + ")</option>"
					}
					me.page.QualList = "<select disabled='disabled' title='" + hEsc(me.textStr("QualityHint")) + "' name='" + App.Proj.Gen.IdQualiList + "' id='" + App.Proj.Gen.IdQualiList + "'>" + me.page.QualList + "</select>"
					me.page.HrefDirLtr = "./" + me.namePage(chapter, quali.SizeHint, pageNr, viewmode, App.Proj.DirModes.Ltr.Name, me.lang, svdt, me.bgCol) + ".html"
					me.page.HrefDirRtl = "./" + me.namePage(chapter, quali.SizeHint, pageNr, viewmode, App.Proj.DirModes.Rtl.Name, me.lang, svdt, me.bgCol) + ".html"

					numFilesWritten += me.genPageExecAndWrite(me.namePage(chapter, quali.SizeHint, pageNr, viewmode, "", me.lang, svdt, me.bgCol), chapter)
				}
			}
		}
	}
	return
}

func (me *siteGen) prepHomePage() {
	s := "<div class='" + App.Proj.Gen.ClsNonViewerPage + "'>"
	cssanimdirs := []string{"alternate-reverse", "alternate"}
	for i, series := range App.Proj.Series {
		if series.Unlisted {
			continue
		}
		author := series.Author
		if author != "" {
			author = strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", strIf(author == "?", me.textStr("Unknown"), author), 1)
		}
		s += "<span class='" + App.Proj.Gen.ClsSeries + "' style='animation-direction: " + cssanimdirs[i%2] + "; background-image: url(\"./" + App.Proj.Gen.PicDirName + "/" + me.nameThumb(series) + ".png\");'><span><h5 id='" + strings.ToLower(series.Name) + "' class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(locStr(series.Title, me.lang)) + "</h5><div class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(locStr(series.Desc, me.lang)) + author + "</div>"
		s += "<ul class='" + App.Proj.Gen.ClsSeries + "'>"
		for _, chapter := range series.Chapters {
			s += "<li class='" + App.Proj.Gen.ClsChapter + "'>"
			if len(chapter.sheets) == 0 {
				s += "<b>" + hEsc(locStr(chapter.Title, me.lang)) + "</b>"
			} else {
				numpages := 1
				if chapter.SheetsPerPage != 0 {
					numpages = len(chapter.sheets) / chapter.SheetsPerPage
				}
				dt1, dt2 := chapter.DateRangeOfSheets()
				sdt1, sdt2 := dt1.Format("Jan 2006"), dt2.Format("Jan 2006")
				sdt := sdt1 + " - " + sdt2
				if sdt1 == sdt2 {
					sdt = sdt1
				}
				title := strings.NewReplacer(
					"%NUMPGS%", itoa(numpages),
					"%NUMPNL%", itoa(chapter.NumPanels()),
					"%NUMSCN%", itoa(chapter.NumScans()),
					"%DATEINFO%", sdt,
				).Replace(me.textStr("ChapStats"))
				if numpages <= 1 {
					title = trim(title[1+strings.IndexByte(title, '/'):])
				}
				if me.lang != App.Proj.Langs[0] {
					if perc := App.Proj.percentTranslated(me.lang, nil, chapter, nil, -1); perc >= 0.0 {
						title += " (" + me.textStr("Transl") + ": " + itoa(int(perc)) + "%)"
					}
				}
				s += "<a title='" + hEsc(title) + "' href='./" + me.namePage(chapter, App.Proj.Qualis[chapter.defaultQuali].SizeHint, 1, "s", "", me.lang, 0, true) + ".html'>" + hEsc(locStr(chapter.Title, me.lang)) + "</a>"
			}
			author := chapter.Author
			if author != "" {
				s += strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", strIf(author == "?", me.textStr("Unknown"), author), 1)
			}
			s += "</li>"
		}
		s += "</ul></span><div></div></span>"
	}
	s += "</div>"
	me.page.PageContent = s
}

func (me *siteGen) prepSheetPage(qIdx int, viewMode string, chapter *Chapter, svDt int64, pageNr int) map[*SheetVer]int {
	quali := App.Proj.Qualis[qIdx]
	me.page.VersList, me.page.ColsList, me.page.ChapTitle, svgTxtCounter = "", "", locStr(chapter.Title, me.lang), 0
	for i, svdt := range chapter.versions {
		var text string
		if i == 0 {
			from, until := time.Unix(0, chapter.verDtLatest.from).Format("January 2006"), time.Unix(0, chapter.verDtLatest.until).Format("January 2006")
			if text = from; from[len(from)-5:] == until[len(until)-5:] && from != until {
				text = from[:len(from)-5] + " - " + until
			} else if from != until {
				text += " - " + until
			}
		} else {
			text = time.Unix(0, svdt).Format("January 2006")
			text += me.textStr("VerOlder")
		}
		if me.lang != App.Proj.Langs[0] {
			for k, v := range App.Proj.PageContentTexts[me.lang] {
				if strings.HasPrefix(k, "Month_") {
					text = strings.Replace(text, k[6:], v, -1)
				}
			}
		}
		me.page.VersList += "<option value='" + me.namePage(chapter, quali.SizeHint, pageNr, viewMode, "", me.lang, svdt, me.bgCol) + "'"
		if svdt == svDt {
			me.page.VersList += " selected='selected'"
		}
		me.page.VersList += ">" + hEsc(text) + "</option>"
	}
	for _, bgcol := range []bool{false, true} {
		if bgcol && !chapter.HasBgCol() {
			continue
		}
		text := me.textStr("Bg" + strIf(!bgcol, "Bw", strIf(chapter.PercentColorized() < 100.0, "ColP", "Col")))
		if perc := chapter.PercentColorized(); bgcol && perc < 100.0 {
			text += " (" + ftoa(perc, 0) + "%)"
		}
		me.page.ColsList += "<option value='" + me.namePage(chapter, quali.SizeHint, pageNr, viewMode, "", me.lang, svDt, bgcol) + "'"
		if bgcol == me.bgCol {
			me.page.ColsList += " selected='selected'"
		}
		me.page.ColsList += ">" + hEsc(text) + "</option>"
	}

	var sheets []*Sheet
	pageslist := func() (s string) {
		istoplist, numpages := (sheets == nil), 1
		switch chapter.SheetsPerPage {
		case 0:
			sheets = chapter.sheets
		default:
			numpages = len(chapter.sheets) / chapter.SheetsPerPage
			var pgnr int
			shownums := map[int]bool{1: true, numpages: true, pageNr: true}
			if !istoplist {
				for i := 1; i <= numpages; i++ {
					shownums[i] = true
				}
			} else {
				for i, want := 1, 4; numpages >= want && len(shownums) < want && i < numpages; i++ {
					if len(shownums) < want && (pageNr+i) < numpages {
						shownums[pageNr+i] = true
					}
					if len(shownums) < want && (pageNr-i) > 1 {
						shownums[pageNr-i] = true
					}
				}
			}
			pglast := -1
			percc := App.Proj.percentTranslated(me.lang, nil, chapter, nil, -1)
			for i := range chapter.sheets {
				if 0 == (i % chapter.SheetsPerPage) {
					pgnr++
					did, name := false, me.namePage(chapter, quali.SizeHint, pgnr, viewMode, "", me.lang, svDt, me.bgCol)
					if did = (pgnr == pageNr); did {
						s += "<li><b><a href='./" + name + ".html'>" + itoa(pgnr) + "</a></b></li>"
					} else if did = shownums[pgnr]; did {
						if perc := App.Proj.percentTranslated(me.lang, nil, chapter, nil, pgnr); perc < 0.0 || perc >= 50 || percc <= 0.0 {
							s += "<li>"
						} else {
							s += "<li class='nolang' title='" + me.lang + ": " + ftoa(perc, 1) + "%'>"
						}
						s += "<a href='./" + name + ".html'>" + itoa(pgnr) + "</a></li>"
					}
					if did {
						pglast = pgnr
					} else if pglast == pgnr-1 {
						s += "<li class='" + App.Proj.Gen.APaging + "s'><span>...&nbsp;</span></li>"
					}
				}
				if pgnr == pageNr && istoplist {
					sheets = append(sheets, chapter.sheets[i])
				}
			}
		}
		nextchap := chapter.NextAfter(true)
		if pageNr == numpages && istoplist && nextchap != nil {
			name := me.namePage(nextchap, quali.SizeHint, 1, viewMode, "", me.lang, 0, me.bgCol)
			s += "<li><a href='./" + name + ".html'>" + locStr(nextchap.Title, me.lang) + "</a></li>"
		}
		if s != "" {
			var pg int
			if pg = pageNr - 1; pg < 1 {
				pg = 1
			}
			pvis, phref := "hidden", me.namePage(chapter, quali.SizeHint, pg, viewMode, "", me.lang, svDt, me.bgCol)
			if pg = pageNr + 1; pg > numpages {
				pg = numpages
			}
			nvis, nhref := "none", me.namePage(chapter, quali.SizeHint, pg, viewMode, "", me.lang, svDt, me.bgCol)
			if pageNr > 1 && istoplist {
				pvis = "visible"
			}
			if pageNr < numpages {
				nvis = "inline-block"
			} else if !istoplist && nextchap != nil {
				nvis, nhref = "inline-block", me.namePage(nextchap, quali.SizeHint, 1, viewMode, "", me.lang, 0, me.bgCol)
			}
			ulid := App.Proj.Gen.APaging
			if !istoplist {
				ulid += "b"
				s = "<ul id='" + ulid + "'>" +
					"<li><a style='display: " + nvis + "' href='./" + strings.ToLower(nhref) + ".html'>&rarr;</a></li>" +
					s +
					"</ul>"
			} else {
				s = "<ul id='" + ulid + "'>" +
					"<li><a style='visibility: " + pvis + "' href='./" + strings.ToLower(phref) + ".html'>&larr;</a></li>" +
					s +
					"<li><a style='display: " + nvis + "' href='./" + strings.ToLower(nhref) + ".html'>&rarr;</a></li>" +
					"</ul>"
			}
		}
		return s
	}
	me.page.PagesList, me.page.PageContent = pageslist(), "<div class='"+App.Proj.Gen.ClsViewerPage+"'>"

	me.page.ViewerList = ""
	for _, viewmode := range viewModes {
		if me.page.ViewerList += "<div title='" + hEsc(me.textStr("ViewMode_"+viewmode)) + "' class='v" + viewmode; viewmode == viewMode {
			me.page.ViewerList += " vc"
		}
		me.page.ViewerList += "'>"
		if n := me.namePage(chapter, quali.SizeHint, pageNr, viewmode, "", me.lang, svDt, me.bgCol); viewmode == viewMode {
			me.page.HrefViewCur = "./" + n + ".html"
			me.page.ViewerList += "<b>&nbsp;</b>"
		} else {
			me.page.HrefViewAlt = "./" + n + ".html"
			me.page.ViewerList += "<a class='" + App.Proj.Gen.ClsPanel + "l' href='" + me.page.HrefViewAlt + "'>&nbsp;</a>"
		}
		me.page.ViewerList += "</div>"
	}

	var iter func(*SheetVer, *ImgPanel, bool) string
	pidx, allpanels, firstpanel, firstrow := 0, map[*SheetVer]int{}, "f", "f"
	iter = func(sv *SheetVer, panel *ImgPanel, istop bool) (s string) {
		assert(len(panel.SubCols) == 0 || len(panel.SubRows) == 0)

		if len(panel.SubRows) > 0 {
			for i := range panel.SubRows {
				sr := &panel.SubRows[i]
				if viewMode == "r" && istop {
					s += "<td>"
				}
				s += "<div id='" + firstrow + App.Proj.Gen.ClsPanel + "r" + sv.id + itoa(i) + "' class='" + App.Proj.Gen.ClsPanelRow
				if firstrow = ""; istop && viewMode == "r" {
					s += " " + App.Proj.Gen.ClsPanelRow + "t"
				} else if istop {
					s += "' onfocus='" + App.Proj.Gen.ClsPanel + "f(this)' tabindex='0"
				}
				s += "'>" + iter(sv, sr, false) + "</div>"
				if viewMode == "r" && istop {
					s += "</td>"
				}
			}

		} else if len(panel.SubCols) > 0 {
			if viewMode == "r" && istop {
				s += "<td>"
			}
			for i := range panel.SubCols {
				sc := &panel.SubCols[i]
				s += "<div class='" + App.Proj.Gen.ClsPanelCol + "'"
				pw, sw := sc.Rect.Dx(), panel.Rect.Dx()
				pp := 100.0 / (float64(sw) / float64(pw))
				s += " style='width: " + ftoa(pp, 8) + "%'"
				s += ">" + iter(sv, sc, false) + "</div>"
			}
			if viewMode == "r" && istop {
				s += "</td>"
			}

		} else {
			allpanels[sv] = pidx
			hqsrc, name := "", me.namePanelPic(sv, pidx, App.Proj.Qualis[0].SizeHint)
			for i := qIdx; i > 0; i-- {
				hqsrc = me.namePanelPic(sv, pidx, App.Proj.Qualis[i].SizeHint) + strIf(App.Proj.Qualis[i].SizeHint == 0, ".svg", ".png")
				if fileinfo := fileStat(".build/" + App.Proj.Gen.PicDirName + "/" + hqsrc); fileinfo != nil && fileinfo.Size() > 0 {
					break
				}
			}
			if len(hqsrc) > 4 && hqsrc[:len(hqsrc)-4] == name {
				hqsrc = ""
			}

			s += "<div id='" + firstpanel + App.Proj.Gen.ClsPanel + "p" + sv.id + itoa(pidx) + "' onclick='" + App.Proj.Gen.ClsPanel + "(this)' class='" + App.Proj.Gen.ClsPanel + "'"
			if firstpanel = ""; viewMode == "r" {
				s += " tabindex='0' onfocus='" + App.Proj.Gen.ClsPanel + "f(this)'"
			}
			s += ">" + me.genSvgForPanel(sv, pidx, panel)
			me.sheetPgNrs[sv] = pageNr
			s += "<img src='./" + App.Proj.Gen.PicDirName + "/" + name + ".png' class='" + App.Proj.Gen.ClsImgHq + "'"
			if hqsrc != "" {
				s += " " + App.Proj.Gen.ClsImgHq + "='" + hqsrc + "'"
			}
			if me.bgCol && sv.data.hasBgCol {
				if bgsvg := fileStat(".build/" + App.Proj.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg" + App.Proj.PanelBgFileExt); bgsvg != nil {
					s += " style='background-image:url(\"./" + App.Proj.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg" + App.Proj.PanelBgFileExt + "\");'"
				}
			}
			s += "/>"
			s += "</div>"
			pidx++
		}
		return
	}
	cls := App.Proj.Gen.ClsSheetsView
	if viewMode == "r" {
		cls = App.Proj.Gen.ClsRowsView
	}
	me.page.PageContent += "<div class='" + App.Proj.Gen.ClsViewer + " " + cls + "'>"
	if viewMode == "r" {
		me.page.PageContent += "<table><tr>"
	}
	for _, sheet := range sheets {
		sheetver := sheet.versions[0]
		if svDt > 0 {
			for i := range sheet.versions {
				if sheet.versions[i].dateTimeUnixNano >= svDt {
					sheetver = sheet.versions[i]
				}
			}
		}
		sheetver.ensurePrep(false, false)
		pidx = 0
		if viewMode != "r" {
			me.page.PageContent += "<div id='" + sheetver.id + "' class='" + App.Proj.Gen.ClsSheet + "'>"
		}
		me.page.PageContent += iter(sheetver, sheetver.data.PanelsTree, true)
		if viewMode != "r" {
			me.page.PageContent += "</div>"
		}
	}
	if viewMode == "r" {
		me.page.PageContent += "</tr></table>"
	}
	me.page.PageContent += "</div>"
	me.page.PageContent += pageslist()
	me.page.PageContent += "</div>"

	return allpanels
}

func (me *siteGen) genSvgForPanel(sV *SheetVer, panelIdx int, panel *ImgPanel) string {
	panelareas, pxcm := sV.panelAreas(panelIdx), sV.data.PxCm
	if pxcm < 472 { // low-dpi special-casing just for the 2 	frog sheets...
		pxcm *= (float64(sV.data.PanelsTree.Rect.Max.X) / 7016.0)
	}
	if len(panelareas) == 0 {
		return ""
	}

	pw, ph := panel.Rect.Dx(), panel.Rect.Dy()
	s := "<svg viewbox='0 0 " + itoa(pw) + " " + itoa(ph) + "'>"
	for _, pta := range panelareas {
		rx, ry, rw, rh := pta.Rect.Min.X-panel.Rect.Min.X, pta.Rect.Min.Y-panel.Rect.Min.Y, pta.Rect.Dx(), pta.Rect.Dy()
		borderandfill := pta.PointTo != nil
		if borderandfill {
			rpx, rpy := pta.PointTo.X-panel.Rect.Min.X, pta.PointTo.Y-panel.Rect.Min.Y
			mmh, cmh := int(pxcm*App.Proj.Gen.PanelSvgText.BoxPolyStrokeWidthCm), int(pxcm/2.0)
			pl, pr, pt, pb := (rx + mmh), ((rx + rw) - mmh), (ry + mmh), ((ry + rh) - mmh)
			poly := [][2]int{{pl, pt}, {pr, pt}, {pr, pb}, {pl, pb}}
			ins := func(idx int, pts ...[2]int) {
				head, tail := poly[:idx], poly[idx:]
				poly = append(head, append(pts, tail...)...)
			}

			if !(pta.PointTo.X == 0 && pta.PointTo.Y == 0) { // "speech-text" pointing somewhere?
				dx, dy := intAbs(rpx-(rx+(rw/2))), intAbs(rpy-(ry+(rh/2)))
				isr, isb := rpx > (rx+(rw/2)), rpy > (ry+(rh/2))
				isl, ist, dst := !isr, !isb, [2]int{rpx, rpy}

				isbr := isb && isr && dy > dx
				isbl := isb && isl && dy > dx
				istr := ist && isr && dy > dx
				istl := ist && isl && dy > dx
				isrb := isr && isb && dx > dy && !isbr
				islb := isl && isb && dx > dy
				isrt := isr && ist && dx > dy
				islt := isl && ist && dx > dy

				if isbl || islb {
					ins(3, [2]int{pl + cmh, pb}, dst)
				} else if isbr || isrb {
					ins(3, dst, [2]int{pr - cmh, pb})
				} else if istr {
					ins(1, [2]int{pr - cmh, pt}, dst)
				} else if istl {
					ins(1, dst, [2]int{pl + cmh, pt})
				} else if isrt {
					ins(2, dst, [2]int{pr, pt + cmh})
				} else if isrb {
					ins(2, [2]int{pr, pb - cmh}, dst)
				} else if islt {
					ins(4, [2]int{pl, pt + cmh}, dst)
				} else if islb {
					ins(4, dst, [2]int{pl, pb - cmh})
				}
			}

			s += "<polygon points='"
			for _, pt := range poly {
				s += itoa(pt[0]) + "," + itoa(pt[1]) + " "
			}
			s += "' class='" + App.Proj.Gen.PanelSvgText.ClsBoxPoly + "' stroke-width='" + itoa(mmh) + "px'/>"
		}
		s += "<svg x='" + itoa(rx) + "' y='" + itoa(ry) + "'>"
		linex := 0.0
		if borderandfill {
			linex = pxcm * App.Proj.Gen.PanelSvgText.BoxPolyDxCmA4
		}
		fontSizeCmA4, perLineDyCmA4 := App.Proj.Gen.PanelSvgText.FontSizeCmA4, App.Proj.Gen.PanelSvgText.PerLineDyCmA4
		if sV.parentSheet.parentChapter.GenPanelSvgText.FontSizeCmA4 > 0.1 { // !=0 in float
			fontSizeCmA4 = sV.parentSheet.parentChapter.GenPanelSvgText.FontSizeCmA4
		}
		if sV.parentSheet.parentChapter.GenPanelSvgText.PerLineDyCmA4 > 0.1 { // !=0 in float
			perLineDyCmA4 = sV.parentSheet.parentChapter.GenPanelSvgText.PerLineDyCmA4
		}
		s += imgSvgText(&pta, me.lang, pxcm, false, int(linex), fontSizeCmA4, perLineDyCmA4)
		s += "</svg>"
	}

	s += "</svg>"
	return s
}

func (me *siteGen) genPageExecAndWrite(name string, chapter *Chapter) (numFilesWritten int) {
	me.page.LangsList = ""
	for lidx, lang := range App.Proj.Langs {
		title, imgsrcpath := lang, strings.Replace(App.Proj.Gen.ImgSrcLang, "%LANG%", lang, -1)
		if langname := App.Proj.PageContentTexts[lang]["LangName"]; langname != "" {
			title = langname
		}
		if lidx != 0 {
			title += " (" + App.Proj.PageContentTexts[lang]["Transl"] + ": " + itoa(int(App.Proj.percentTranslated(lang, nil, nil, nil, -1))) + "%"
			if chapter != nil {
				title += ", \"" + locStr(chapter.Title, lang) + "\": " + itoa(int(App.Proj.percentTranslated(lang, nil, chapter, nil, -1))) + "%"
			}
			title += ")"
		}
		if lang == me.lang {
			me.page.LangsList += "<span><div>"
			me.page.LangsList += "<b><img title='" + hEsc(title) + "' alt='" + hEsc(title) + "' src='" + imgsrcpath + "'/></b>"
			me.page.LangsList += "</div></span>"
		} else {
			me.page.LangsList += "<div>"
			href := name[:len(name)-len(me.lang)] + lang
			if strings.HasPrefix(name, "index") {
				if me.lang == App.Proj.Langs[0] {
					href = name + "." + lang
				} else if lang == App.Proj.Langs[0] {
					var dirmode string
					if me.dirRtl {
						dirmode = "." + App.Proj.DirModes.Rtl.Name
					}
					href = "index" + dirmode
				}
			}
			me.page.LangsList += "<a class='" + App.Proj.Gen.ClsPanel + "l' href='./" + href + ".html'><img alt='" + hEsc(title) + "' title='" + hEsc(title) + "' src='" + imgsrcpath + "'/></a>"
			me.page.LangsList += "</div>"
		}
	}
	if me.page.PageTitleTxt == "" {
		me.page.PageTitleTxt = me.page.PageTitle
	}
	if me.dirRtl {
		me.page.HrefDirCur, me.page.HrefDirAlt = me.page.HrefDirRtl, me.page.HrefDirLtr
		me.page.DirCurTitle, me.page.DirAltTitle = locStr(App.Proj.DirModes.Rtl.Title, me.lang), locStr(App.Proj.DirModes.Ltr.Title, me.lang)
		me.page.DirCurDesc, me.page.DirAltDesc = locStr(App.Proj.DirModes.Rtl.Desc, me.lang), locStr(App.Proj.DirModes.Ltr.Desc, me.lang)
	} else {
		me.page.HrefDirCur, me.page.HrefDirAlt = me.page.HrefDirLtr, me.page.HrefDirRtl
		me.page.DirCurTitle, me.page.DirAltTitle = locStr(App.Proj.DirModes.Ltr.Title, me.lang), locStr(App.Proj.DirModes.Rtl.Title, me.lang)
		me.page.DirCurDesc, me.page.DirAltDesc = locStr(App.Proj.DirModes.Ltr.Desc, me.lang), locStr(App.Proj.DirModes.Rtl.Desc, me.lang)
	}

	buf := bytes.NewBuffer(nil)
	if err := me.tmpl.ExecuteTemplate(buf, "site.html", &me.page); err != nil {
		panic(err)
	}
	fileWrite(".build/"+strings.ToLower(name)+".html", buf.Bytes())
	numFilesWritten++
	return
}

func (me *siteGen) textStr(key string) (s string) {
	if s = App.Proj.PageContentTexts[me.lang][key]; s == "" {
		if s = App.Proj.PageContentTexts[App.Proj.Langs[0]][key]; s == "" {
			s = key
		}
	}
	return s
}

func (me *siteGen) genAtomXml() (numFilesWritten int) {
	af, tlatest := App.Proj.AtomFile, ""
	if len(af.PubDates) == 0 {
		return
	}
	var xmls []string
	for i, pubdate := range af.PubDates {
		entryidx, nextolderdate := 0, "0000-00-00"
		if i < len(af.PubDates)-1 {
			nextolderdate = af.PubDates[i+1]
		}
		for _, series := range App.Proj.Series {
			for _, chapter := range series.Chapters {
				pgnr, numpanels, numsheets, pages := -1, 0, 0, map[int]bool{}
				for _, sheet := range chapter.sheets {
					for _, sv := range sheet.versions {
						if dtstr := time.Unix(0, sv.dateTimeUnixNano).Format("2006-01-02"); dtstr > nextolderdate && dtstr <= pubdate {
							pg := me.sheetPgNrs[sv]
							pages[pg] = true
							npnl, _ := sv.panelCount()
							numsheets, numpanels = numsheets+1, numpanels+npnl
							if pgnr == -1 {
								if pgnr = 1; pg > 0 {
									pgnr = pg
								}
							}
							if pg > 0 && pg < pgnr {
								pgnr = pg
							}
						}
					}
				}
				if tpub := pubdate + `T00:00:0` + itoa(entryidx); pgnr >= 1 {
					if entryidx++; tlatest == "" || tpub > tlatest {
						tlatest = tpub
					}
					href := "http://" + strings.TrimRight(strings.ToLower(SiteTitleEsc), "/") + "/" + me.namePage(chapter, App.Proj.Qualis[chapter.defaultQuali].SizeHint, pgnr, "s", "", me.lang, 0, true) + ".html"
					xml := `<entry><updated>` + tpub + `Z</updated>`
					xml += `<title>` + hEsc(locStr(chapter.parentSeries.Title, me.lang)) + `: ` + hEsc(locStr(chapter.Title, me.lang)) + `</title>`
					xml += `<id>` + href + `</id><link href="` + href + `"/>`
					xml += `<author><name>` + SiteTitleEsc + `</name></author>`
					xml += `<content type="html">` + strings.NewReplacer(
						"%NUMSVS%", itoa(numsheets),
						"%NUMPNL%", itoa(numpanels),
						"%NUMPGS%", itoa(len(pages)),
					).Replace(locStr(af.ContentHtml, me.lang)) + `</content>`
					xmls = append(xmls, xml+`</entry>`)
				}
			}
		}
	}

	filename := af.Name + "." + me.lang + ".atom"
	s := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom" xml:lang="` + me.lang + `">`
	if len(xmls) > 0 && tlatest != "" {
		s += `<updated>` + tlatest + `Z</updated><title>` + SiteTitleEsc + `</title><link href="http://` + strings.ToLower(SiteTitleEsc) + `"/><link rel="self" href="http://` + strings.ToLower(SiteTitleEsc) + `/` + filename + `"/><id>http://` + strings.ToLower(SiteTitleEsc) + "</id>"
		s += "\n" + strings.Join(xmls, "\n")
	}
	fileWrite(".build/"+af.Name+"."+me.lang+".atom", []byte(s+"\n</feed>"))
	numFilesWritten++
	return
}

func (me *siteGen) copyHomeThumbsPngs() (numPngs uint32) {
	for _, series := range App.Proj.Series {
		thumbfilename := me.nameThumb(series) + ".png"
		if srcfilepath, dstfilepath := ".ccache/"+thumbfilename, ".build/"+App.Proj.Gen.PicDirName+"/"+thumbfilename; fileStat(srcfilepath) != nil {
			numPngs++
			fileLinkOrCopy(srcfilepath, dstfilepath)
		}
	}
	return
}

func (siteGen) namePanelPic(sheetVer *SheetVer, pIdx int, qualiSizeHint int) string {
	return sheetVer.DtStr() + sheetVer.id + itoa(pIdx) + "." + itoa(qualiSizeHint)
}

func (siteGen) nameThumb(series *Series) string {
	return "_" + App.Proj.DirModes.Ltr.Name + "-" + App.Proj.DirModes.Rtl.Name + "-" + strings.ToLower(series.UrlName) + "." + itoa(App.Proj.NumSheetsInHomeBgs)
}

func (me *siteGen) namePage(chapter *Chapter, qualiSizeHint int, pageNr int, viewMode string, dirMode string, langId string, svDt int64, bgCol bool) string {
	if pageNr < 1 {
		pageNr = 1
	}
	if dirMode == "" {
		if dirMode = App.Proj.DirModes.Ltr.Name; me.dirRtl {
			dirMode = App.Proj.DirModes.Rtl.Name
		}
	}
	return strings.ToLower(chapter.parentSeries.UrlName + "-" + chapter.UrlName + "-" + itoa(pageNr) + strIf(bgCol && chapter.HasBgCol(), "col", "bw") + strconv.FormatInt(svDt, 36) + viewMode + itoa(qualiSizeHint) + "-" + dirMode + "." + langId)
}
