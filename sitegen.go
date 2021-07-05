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

var viewModes = []string{"s", "r"}

type PageGen struct {
	SiteTitle      string
	SiteDesc       string
	PageTitle      string
	PageTitleTxt   string
	PageDesc       string
	PageLang       string
	PageCssClasses string
	PageDirCur     string
	PageDirAlt     string
	DirCurTitle    string
	DirAltTitle    string
	DirCurDesc     string
	DirAltDesc     string
	LangsList      string
	ViewerList     string
	HrefViewAlt    string
	HrefViewCur    string
	QualList       string
	PagesList      string
	PageContent    string
	HintHtmlR      string
	HintHtmlS      string
	HintDir        string
	VerHint        string
	LegalHtml      string
	HrefHome       string
	HrefDirLtr     string
	HrefDirRtl     string
	HrefDirCur     string
	HrefDirAlt     string
	HrefFeed       string
	VersList       string
	ChapTitle      string
}

type siteGen struct {
	tmpl       *template.Template
	page       PageGen
	lang       string
	dirRtl     bool
	onPicSize  func(*Chapter, string, int, int64)
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
		numsvgs, numpngs, numsheets, numpanels := me.genOrCopyPanelPics()
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
		return "for " + itoa(int(numpngs)) + " PNGs & " + itoa(int(numsvgs)) + " SVGs from " + itoa(int(numpanels)) + " panels in " + itoa(int(numsheets)) + " sheets"
	})

	timedLogged("SiteGen: generating markup files...", func() string {
		numfileswritten := 0
		me.tmpl, err = template.New("foo").ParseFiles(siteTmplDirName + "/site.html")
		if err != nil {
			panic(err)
		}
		for _, me.lang = range App.Proj.Langs {
			for _, me.dirRtl = range []bool{true, false /*keep this order of bools*/} {
				numfileswritten += me.genPages(nil, 0)
				for _, series := range App.Proj.Series {
					for _, chapter := range series.Chapters {
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
						if csssel != "" {
							if csslines == nil {
								csslines = App.Proj.Gen.PanelSvgText.Css[""]
							}
							data = append([]byte(csssel+"{"+strings.Join(csslines, ";")+"}\n"), data...)
						}
					}
				}
				fileWrite(dstpath, data)
				numFilesWritten++
			}
		}
	}
	return
}

func (me *siteGen) genOrCopyPanelPics() (numSvgs uint32, numPngs uint32, numSheets uint32, numPanels uint32) {
	var work sync.WaitGroup
	atomic.StoreUint32(&numSheets, 0)
	atomic.StoreUint32(&numPngs, 0)
	atomic.StoreUint32(&numSvgs, 0)
	atomic.StoreUint32(&numPanels, 0)
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			for _, sheet := range chapter.sheets {
				work.Add(1)
				go func(chapter *Chapter, sheet *Sheet) {
					for _, sv := range sheet.versions {
						nsvgs, npngs, npnls := me.genOrCopyPanelPicsOf(sv)
						atomic.AddUint32(&numSheets, 1)
						atomic.AddUint32(&numPngs, npngs)
						atomic.AddUint32(&numPanels, npnls)
						atomic.AddUint32(&numSvgs, nsvgs)
					}
					work.Done()
				}(chapter, sheet)
			}
		}
	}
	work.Wait()
	return
}

func (me *siteGen) genOrCopyPanelPicsOf(sv *SheetVer) (numSvgs uint32, numPngs uint32, numPanels uint32) {
	sv.ensurePrep(false, false)
	atomic.StoreUint32(&numPngs, 0)
	atomic.StoreUint32(&numSvgs, 0)
	var pidx int
	var work sync.WaitGroup
	sv.data.PanelsTree.iter(func(panel *ImgPanel) {
		work.Add(1)
		numPanels++
		go func(pidx int) {
			for qidx, quali := range App.Proj.Qualis {
				fext := strIf(quali.SizeHint == 0, ".svg", ".png")
				srcpath := filepath.Join(sv.data.PicDirPath(quali.SizeHint), itoa(pidx)+fext)
				if fileinfo := fileStat(srcpath); fileinfo == nil && quali.SizeHint != 0 {
					break
				} else {
					dstpath := filepath.Join(".build/"+App.Proj.Gen.PicDirName+"/", me.namePanelPic(sv.id, pidx, quali.SizeHint)+fext)
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
			srcpath := filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".svg")
			if fileinfo := fileStat(srcpath); fileinfo != nil {
				dstpath := filepath.Join(".build/" + App.Proj.Gen.PicDirName + "/" + sv.id + itoa(pidx) + "bg.svg")
				fileLinkOrCopy(srcpath, dstpath)
				atomic.AddUint32(&numSvgs, 1)
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
		SiteTitle:  hEsc(App.Proj.Title),
		SiteDesc:   repl.Replace(hEsc(locStr(App.Proj.Desc, me.lang))),
		PageLang:   me.lang,
		HintHtmlR:  me.textStr("HintHtmlR"),
		HintHtmlS:  me.textStr("HintHtmlS"),
		HintDir:    me.textStr("HintDir"),
		VerHint:    me.textStr("VerHint"),
		LegalHtml:  me.textStr("LegalHtml"),
		HrefFeed:   "./" + App.Proj.AtomFile.Name + "." + me.lang + ".atom",
		PageDirCur: "ltr",
		PageDirAlt: "rtl",
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
		me.page.PageCssClasses = App.Proj.Gen.ClsChapter + "n"
		if me.lang == App.Proj.Langs[0] {
			me.page.HrefDirLtr = "./index.html"
			me.page.HrefDirRtl = "./index." + App.Proj.DirModes.Rtl.Name + ".html"
		} else {
			me.page.HrefDirLtr = "./index." + me.lang + ".html"
			me.page.HrefDirRtl = "./index." + App.Proj.DirModes.Rtl.Name + "." + me.lang + ".html"
		}
		me.prepHomePage()
		numFilesWritten += me.genPageExecAndWrite(homename)

	} else {
		series := chapter.parentSeries
		me.page.HrefHome += "#" + strings.ToLower(series.Name)
		me.page.PageTitle = "<span>" + hEsc(locStr(series.Title, me.lang)) + " &bull;</span> " + hEsc(locStr(chapter.Title, me.lang))
		me.page.PageTitleTxt = hEsc(locStr(series.Title, me.lang)) + ": " + hEsc(locStr(chapter.Title, me.lang))
		author := series.Author
		if author != "" {
			author = strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", author, 1)
		}
		me.page.PageDesc = hEsc(locStr(series.Desc, me.lang)) + author
		for qidx, quali := range App.Proj.Qualis {
			for _, viewmode := range viewModes {
				me.page.PageCssClasses = App.Proj.Gen.ClsChapter + viewmode
				for _, svdt := range chapter.versions {
					qname, qsizes, allpanels := quali.Name, map[int]int64{}, me.prepSheetPage(qidx, viewmode, chapter, svdt, pageNr)
					me.page.QualList = ""
					for i, q := range App.Proj.Qualis {
						var totalimgsize int64
						for contenthash, maxpidx := range allpanels {
							if bgfile := fileStat(".build/" + App.Proj.Gen.PicDirName + "/" + contenthash + "bg.svg"); bgfile != nil {
								totalimgsize += bgfile.Size()
							}
							for pidx := 0; pidx <= maxpidx; pidx++ {
								name := me.namePanelPic(contenthash, pidx, q.SizeHint)
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
						me.page.QualList += "<option value='" + me.namePage(chapter, q.SizeHint, pageNr, viewmode, "", me.lang, svdt) + "'"
						if q.Name == qname {
							me.page.QualList += " selected='selected'"
						}
						me.page.QualList += ">" + q.Name + " (" + strSize64(qsizes[i]) + ")</option>"
					}
					me.page.QualList = "<select disabled='disabled' title='" + hEsc(me.textStr("QualityHint")) + "' name='" + App.Proj.Gen.IdQualiList + "' id='" + App.Proj.Gen.IdQualiList + "'>" + me.page.QualList + "</select>"
					me.page.HrefDirLtr = "./" + me.namePage(chapter, quali.SizeHint, pageNr, viewmode, App.Proj.DirModes.Ltr.Name, me.lang, svdt) + ".html"
					me.page.HrefDirRtl = "./" + me.namePage(chapter, quali.SizeHint, pageNr, viewmode, App.Proj.DirModes.Rtl.Name, me.lang, svdt) + ".html"

					numFilesWritten += me.genPageExecAndWrite(me.namePage(chapter, quali.SizeHint, pageNr, viewmode, "", me.lang, svdt))
				}
			}
		}
	}
	return
}

func (me *siteGen) prepHomePage() {
	s := "<div class='" + App.Proj.Gen.ClsNonViewerPage + "'>"
	cssanimdirs := []string{"alternate", "alternate-reverse"}
	for i, series := range App.Proj.Series {
		authorinfo := series.Author
		if authorinfo == "" {
			authorinfo = me.textStr("Unknown")
		}
		if authorinfo != "" {
			authorinfo = strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", authorinfo, 1)
		}
		s += "<span class='" + App.Proj.Gen.ClsSeries + "' style='animation-direction: " + cssanimdirs[i%2] + "; background-image: url(\"./" + App.Proj.Gen.PicDirName + "/" + me.nameThumb(series) + ".png\");'><span><h5 id='" + strings.ToLower(series.Name) + "' class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(locStr(series.Title, me.lang)) + "</h5><div class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(locStr(series.Desc, me.lang)) + authorinfo + "</div>"
		s += "<ul class='" + App.Proj.Gen.ClsSeries + "'>"
		for _, chapter := range series.Chapters {
			s += "<li class='" + App.Proj.Gen.ClsChapter + "'>"
			if len(chapter.sheets) > 0 {
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
						title = "(" + me.textStr("Transl") + ": " + itoa(int(perc)) + "%) " + title
					}
				}
				s += "<a title='" + hEsc(title) + "' href='./" + me.namePage(chapter, App.Proj.Qualis[chapter.defaultQuali].SizeHint, 1, "s", "", me.lang, 0) + ".html'>" + hEsc(locStr(chapter.Title, me.lang)) + "</a>"
			} else {
				s += "<b>" + hEsc(locStr(chapter.Title, me.lang)) + "</b>"
			}
			s += "</li>"
		}
		s += "</ul></span><div></div></span>"
	}
	s += "</div>"
	me.page.PageContent = s
}

func (me *siteGen) prepSheetPage(qIdx int, viewMode string, chapter *Chapter, svDt int64, pageNr int) map[string]int {
	quali := App.Proj.Qualis[qIdx]
	me.page.VersList, me.page.ChapTitle, svgTxtCounter = "", locStr(chapter.Title, me.lang), 0
	for i, svdt := range chapter.versions {
		var text string
		if i == 0 {
			from, until := time.Unix(0, chapter.verDtLatest.from).Format("January 2006"), time.Unix(0, chapter.verDtLatest.until).Format("January 2006")
			if text = from; from[len(from)-5:] == until[len(until)-5:] && from != until {
				text = from[:len(from)-5] + " - " + until
			} else if from != until {
				text += " - " + until
			}
			text += me.textStr("VerNewest")
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
		me.page.VersList += "<option value='" + me.namePage(chapter, quali.SizeHint, pageNr, viewMode, "", me.lang, svdt) + "'"
		if svdt == svDt {
			me.page.VersList += " selected='selected'"
		}
		me.page.VersList += ">" + hEsc(text) + "</option>"
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
			for i := range chapter.sheets {
				if 0 == (i % chapter.SheetsPerPage) {
					pgnr++
					did, name := false, me.namePage(chapter, quali.SizeHint, pgnr, viewMode, "", me.lang, svDt)
					if did = (pgnr == pageNr); did {
						s += "<li><b><a href='./" + name + ".html'>" + itoa(pgnr) + "</a></b></li>"
					} else if did = shownums[pgnr]; did {
						if perc := App.Proj.percentTranslated(me.lang, nil, chapter, nil, pgnr); perc < 0.0 || perc >= 99.9 {
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
			name := me.namePage(nextchap, quali.SizeHint, 1, viewMode, "", me.lang, svDt)
			s += "<li><a href='./" + name + ".html'>" + locStr(nextchap.Title, me.lang) + "</a></li>"
		}
		if s != "" {
			var pg int
			if pg = pageNr - 1; pg < 1 {
				pg = 1
			}
			pvis, phref := "hidden", me.namePage(chapter, quali.SizeHint, pg, viewMode, "", me.lang, svDt)
			if pg = pageNr + 1; pg > numpages {
				pg = numpages
			}
			nvis, nhref := "none", me.namePage(chapter, quali.SizeHint, pg, viewMode, "", me.lang, svDt)
			if pageNr > 1 && istoplist {
				pvis = "visible"
			}
			if pageNr < numpages {
				nvis = "inline-block"
			} else if !istoplist && nextchap != nil {
				nvis, nhref = "inline-block", me.namePage(nextchap, quali.SizeHint, 1, viewMode, "", me.lang, svDt)
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
		if n := me.namePage(chapter, quali.SizeHint, pageNr, viewmode, "", me.lang, svDt); viewmode == viewMode {
			me.page.HrefViewCur = "./" + n + ".html"
			me.page.ViewerList += "<b>&nbsp;</b>"
		} else {
			me.page.HrefViewAlt = "./" + n + ".html"
			me.page.ViewerList += "<a class='" + App.Proj.Gen.ClsPanel + "l' href='" + me.page.HrefViewAlt + "'>&nbsp;</a>"
		}
		me.page.ViewerList += "</div>"
	}

	var iter func(*SheetVer, *ImgPanel, bool) string
	pidx, allpanels, firstpanel, firstrow := 0, map[string]int{}, "f", "f"
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
				pw, sw := sc.Rect.Max.X-sc.Rect.Min.X, panel.Rect.Max.X-panel.Rect.Min.X
				pp := 100.0 / (float64(sw) / float64(pw))
				s += " style='width: " + ftoa(pp, 8) + "%'"
				s += ">" + iter(sv, sc, false) + "</div>"
			}
			if viewMode == "r" && istop {
				s += "</td>"
			}

		} else {
			allpanels[sv.id] = pidx
			hqsrc, name := "", me.namePanelPic(sv.id, pidx, App.Proj.Qualis[0].SizeHint)
			for i := qIdx; i > 0; i-- {
				hqsrc = me.namePanelPic(sv.id, pidx, App.Proj.Qualis[i].SizeHint) + strIf(App.Proj.Qualis[i].SizeHint == 0, ".svg", ".png")
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
			if bgsvg := fileStat(".build/" + App.Proj.Gen.PicDirName + "/" + sv.id + "bg.svg"); bgsvg != nil {
				s += " style='background-image:url(\"./" + App.Proj.Gen.PicDirName + "/" + sv.id + "bg.svg\");'"
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

	pw, ph := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y
	s := "<svg viewbox='0 0 " + itoa(pw) + " " + itoa(ph) + "'>"
	for _, pta := range panelareas {
		rx, ry, rw, rh := pta.Rect.Min.X-panel.Rect.Min.X, pta.Rect.Min.Y-panel.Rect.Min.Y, pta.Rect.Max.X-pta.Rect.Min.X, pta.Rect.Max.Y-pta.Rect.Min.Y
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
				if isbl := (isb && isl && dy >= dx); isbl {
					ins(3, [2]int{pl + cmh, pb}, dst)
				} else if isbr := (isb && isr && dy >= dx); isbr {
					ins(3, dst, [2]int{pr - cmh, pb})
				} else if istr := (ist && isr && dy >= dx); istr {
					ins(1, [2]int{pr - cmh, pt}, dst)
				} else if istl := (ist && isl && dy >= dx); istl {
					ins(1, dst, [2]int{pl + cmh, pt})
				} else if isrb := (isr && isb && dx >= dy); isrb {
					ins(2, [2]int{pr, pb - cmh}, dst)
				} else if isrt := (isr && ist && dx >= dy); isrt {
					ins(2, dst, [2]int{pr, pt + cmh})
				} else if islt := (isl && ist && dx >= dy); islt {
					ins(4, [2]int{pl, pt + cmh}, dst)
				} else if islb := (isl && isb && dx >= dy); islb {
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
		s += imgSvgText(&pta, me.lang, pxcm, false, int(linex))
		s += "</svg>"
	}

	s += "</svg>"
	return s
}

func (me *siteGen) genPageExecAndWrite(name string) (numFilesWritten int) {
	me.page.LangsList = ""
	for lidx, lang := range App.Proj.Langs {
		title, imgsrcpath := lang, strings.Replace(App.Proj.Gen.ImgSrcLang, "%LANG%", lang, -1)
		if lidx != 0 {
			title += " (" + App.Proj.PageContentTexts[lang]["Transl"] + ": " + itoa(int(App.Proj.percentTranslated(lang, nil, nil, nil, -1))) + "%)"
		}
		if lang == me.lang {
			me.page.LangsList += "<span><div>"
			me.page.LangsList += "<b><img title='" + title + "' alt='" + title + "' src='" + imgsrcpath + "'/></b>"
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
			me.page.LangsList += "<a class='" + App.Proj.Gen.ClsPanel + "l' href='./" + href + ".html'><img alt='" + title + "' title='" + title + "' src='" + imgsrcpath + "'/></a>"
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
	af := App.Proj.AtomFile
	if len(af.PubDates) == 0 {
		return
	}
	var xmls []string
	for i, pubdate := range af.PubDates {
		nextolderdate := "0000-00-00"
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
				if pgnr >= 1 {
					xml := `<entry><updated>` + pubdate + `T00:00:00Z</updated>`
					xml += `<title>` + hEsc(locStr(chapter.parentSeries.Title, me.lang)) + `: ` + hEsc(locStr(chapter.Title, me.lang)) + `</title>`
					xml += `<link href="` + strings.TrimRight(af.LinkHref, "/") + "/" + me.namePage(chapter, App.Proj.Qualis[chapter.defaultQuali].SizeHint, pgnr, "s", "", me.lang, 0) + ".html" + `"/>`
					xml += `<author><name>` + af.Title + `</name></author>`
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

	s := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom" xml:lang="` + me.lang + `">`
	if len(xmls) > 0 {
		s += `<updated>` + af.PubDates[0] + `T00:00:00Z</updated><title>` + af.Title + `</title><link href="` + af.LinkHref + `"/><id>` + af.LinkHref + "</id>"
		s += "\n" + strings.Join(xmls, "\n")
	}
	fileWrite(".build/"+af.Name+"."+me.lang+".atom", []byte(s+"\n</feed>"))
	numFilesWritten++
	return
}

func (me *siteGen) copyHomeThumbsPngs() (numPngs uint32) {
	for _, series := range App.Proj.Series {
		thumbfilename := me.nameThumb(series) + ".png"
		if srcfilepath, dstfilepath := ".cache/"+thumbfilename, ".build/"+App.Proj.Gen.PicDirName+"/"+thumbfilename; fileStat(srcfilepath) != nil {
			numPngs++
			fileLinkOrCopy(srcfilepath, dstfilepath)
		}
	}
	return
}

func (siteGen) namePanelPic(sheetId string, pIdx int, qualiSizeHint int) string {
	return sheetId + itoa(pIdx) + itoa(qualiSizeHint)
}

func (siteGen) nameThumb(series *Series) string {
	return "_" + App.Proj.DirModes.Ltr.Name + "-" + App.Proj.DirModes.Rtl.Name + "-" + strings.ToLower(series.UrlName)
}

func (me *siteGen) namePage(chapter *Chapter, qualiSizeHint int, pageNr int, viewMode string, dirMode string, langId string, svDt int64) string {
	if pageNr < 1 {
		pageNr = 1
	}
	if dirMode == "" {
		if dirMode = App.Proj.DirModes.Ltr.Name; me.dirRtl {
			dirMode = App.Proj.DirModes.Rtl.Name
		}
	}
	return strings.ToLower(chapter.parentSeries.UrlName + "-" + chapter.UrlName + "-" + itoa(pageNr) + strconv.FormatInt(svDt, 36) + viewMode + itoa(qualiSizeHint) + "-" + dirMode + "." + langId)
}
