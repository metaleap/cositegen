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
	"text/template/parse"
	"time"
)

var viewModes = []string{"s", "r"}

type siteGen struct {
	tmpl       *template.Template
	series     []*Series
	page       PageGen
	lang       string
	bgCol      bool
	dirRtl     bool
	onPicSize  func(*Chapter, string, int, int64)
	maxPicSize uint32
	sheetPgNrs map[*SheetVer]int
}

type PageGen struct {
	SiteTitle      string
	SiteTitleEsc   string
	SiteDesc       string
	SiteHost       string
	PageTitle      string
	PageTitleTxt   string
	PageDesc       string
	PageDescTxt    string
	PageLang       string
	PageCssClasses string
	PageDirCur     string
	PageDirAlt     string
	DirCurTitle    string
	DirAltTitle    string
	LangsList      string
	ViewerList     string
	HrefViewAlt    string
	HrefViewCur    string
	QualList       string
	PagesList      string
	PageContent    string
	HrefHome       string
	HrefDirLtr     string
	HrefDirRtl     string
	HrefDirCur     string
	HrefDirAlt     string
	HrefFeed       string
	VersList       string
	ColsList       string
	ChapTitle      string
}

func (me siteGen) genSite(fromGui bool, flags map[string]bool) {
	var err error
	tstart := time.Now()
	me.series = App.Proj.Series
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
	mkDir(".build/" + App.Proj.Site.Gen.PicDirName)

	timedLogged("SiteGen: copying static files to .build...", func() string {
		if App.Proj.Sheets.Panel.SvgText.AppendToFiles == nil {
			App.Proj.Sheets.Panel.SvgText.AppendToFiles = map[string]bool{}
		}
		numfilescopied := me.copyStaticFiles("")
		return "for " + itoa(numfilescopied) + " files"
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
			for qidx, size := range chq {
				if App.Proj.Qualis[qidx].ExcludeInSiteGen {
					continue
				}
				if (min == 0 || size < min) && App.Proj.Qualis[qidx].SizeHint <= 4096 && App.Proj.Qualis[qidx].SizeHint > 0 {
					min = size
				}
				pref := chapter.Name
				if qidx > 0 {
					pref = strings.Repeat(" ", len(pref))
				}
				printLn("\t\t" + pref + "\t\t" + App.Proj.Qualis[qidx].Name + "(" + itoa(App.Proj.Qualis[qidx].SizeHint) + ") => " + strSize(uint64(size)))
			}
		}
		return "for " + itoa(int(numpngs)) + " PNGs & " + itoa(int(numsvgs)) + " SVGs (" + strSize(totalsize) + ") from " + itoa(int(numpanels)) + " panels in " + itoa(int(numsheets)) + " sheets, max panel pic size: " + strSize(uint64(me.maxPicSize))
	})

	timedLogged("SiteGen: generating markup files...", func() string {
		numfileswritten := 0
		var totalsize uint64
		me.tmpl = template.New("site.html")
		me.tmpl.Funcs(template.FuncMap{"__": me.textStr})
		me.tmpl, err = me.tmpl.ParseFiles(siteTmplDirName + "/site.html")
		me.tmpl.Mode = parse.SkipFuncCheck
		if err != nil {
			panic(err)
		}
		for _, me.lang = range App.Proj.Langs {
			for _, me.dirRtl = range []bool{true, false /*KEEP this order of bools*/} {
				me.bgCol = false
				numfileswritten += me.genPages(nil, 0, &totalsize)
				for _, me.bgCol = range []bool{false, true} {
					for _, series := range me.series {
						for _, chapter := range series.Chapters {
							if me.bgCol && !chapter.HasBgCol() {
								continue
							}
							for i := range chapter.SheetsPerPage {
								numfileswritten += me.genPages(chapter, 1+i, &totalsize)
							}
						}
					}
				}
				if App.Proj.Site.Feed.Name != "" {
					numfileswritten += me.genAtomXml(&totalsize)
				}
				for _, series := range me.series {
					for _, chapter := range series.Chapters {
						numfileswritten++
						totalsize += uint64(len(me.genSvgTextsFile(chapter)))
					}
				}
			}
		}
		return "for " + itoa(numfileswritten) + " files (~" + strSize(totalsize) + ")"
	})

	printLn("SiteGen: " + App.Proj.Site.Host + " DONE after " + time.Now().Sub(tstart).String())
	cmd := exec.Command(browserCmd[0], append(browserCmd[1:], "--app=file://"+os.Getenv("PWD")+"/.build/index.html")...)
	if err := cmd.Start(); err != nil {
		printLn("[ERR]\tcmd.Start of " + cmd.String() + ":\t" + err.Error())
	} else {
		go func() {
			if err := cmd.Wait(); err != nil {
				printLn("[ERR]\tcmd.Wait of " + cmd.String() + ":\t" + err.Error())
			}
		}()
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
				if App.Proj.Sheets.Panel.SvgText.AppendToFiles[relpath] {
					for csssel, csslines := range App.Proj.Sheets.Panel.SvgText.Css {
						if csssel != "" && csssel != "@font-face" {
							if csslines == nil {
								csslines = App.Proj.Sheets.Panel.SvgText.Css[""]
							}
							data = append([]byte(csssel+"{"+strings.Join(csslines, ";")+"}\n"), data...)
						}
					}
					if cssff := App.Proj.Sheets.Panel.SvgText.Css["@font-face"]; len(cssff) != 0 {
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
	_ = sv.ensurePrep(false, false)
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
				if quali.ExcludeInSiteGen {
					continue
				}
				fext := sIf(quali.SizeHint == 0, ".svg", ".png")
				srcpath := filepath.Join(sv.data.PicDirPath(quali.SizeHint), itoa(pidx)+fext)
				if fileinfo := fileStat(srcpath); fileinfo == nil && quali.SizeHint != 0 {
					break
				} else {
					for fs, swap := uint32(fileinfo.Size()), true; swap; {
						max := atomic.LoadUint32(&me.maxPicSize)
						swap = fs > max && !atomic.CompareAndSwapUint32(&me.maxPicSize, max, fs)
					}
					atomic.AddUint64(&totalSize, uint64(fileinfo.Size()))
					dstpath := filepath.Join(".build/"+App.Proj.Site.Gen.PicDirName+"/", me.namePanelPic(sv, pidx, quali.SizeHint)+fext)
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
			if srcpath := filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".png"); sv.data.hasBgCol {
				if fileinfo := fileStat(srcpath); fileinfo != nil {
					atomic.AddUint64(&totalSize, uint64(fileinfo.Size()))
					dstpath := filepath.Join(".build/" + App.Proj.Site.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg.png")
					fileLinkOrCopy(srcpath, dstpath)
					atomic.AddUint32(&numPngs, 1)
				}
			}
			work.Done()
		}(pidx)
		pidx++
	})

	work.Wait()
	return
}

func (me *siteGen) genPages(chapter *Chapter, pageNr int, totalSizeRec *uint64) (numFilesWritten int) {
	homename, repl := me.namePage(nil, 0, 0, "", "", "", 0, false), strings.NewReplacer(
		"%LANG"+me.lang+"%", itoa(int(App.Proj.percentTranslated(me.lang, nil, nil, nil, -1))),
	)
	me.page = PageGen{
		SiteTitle:    App.Proj.Site.Title,
		SiteTitleEsc: hEsc(App.Proj.Site.Title),
		SiteHost:     App.Proj.Site.Host,
		SiteDesc:     repl.Replace(hEsc(locStr(App.Proj.Site.Desc, me.lang))),
		PageLang:     me.lang,
		HrefFeed:     "./" + App.Proj.Site.Feed.Name + "." + me.lang + ".atom",
		PageDirCur:   "ltr",
		PageDirAlt:   "rtl",
	}
	if parts := strings.Split(trim(me.page.SiteDesc)+" ", ". "); len(parts) > 1 {
		for i, s := range parts {
			parts[i] = sIf(s == "", "", "<nobr>"+s+".</nobr> ")
		}
		me.page.SiteDesc = trim(strings.Join(parts, ""))
	}
	if me.dirRtl {
		me.page.PageDirCur, me.page.PageDirAlt = "rtl", "ltr"
	}
	me.page.HrefHome = "./" + homename + ".html"

	if chapter == nil {
		me.page.PageTitle = hEsc(me.textStr("HomeTitle"))
		me.page.PageTitleTxt = hEsc(me.textStr("HomeTitleTxt"))
		me.page.PageDesc = repl.Replace(hEsc(me.textStr("HomeDesc")))
		me.page.PageDescTxt = me.page.PageDesc
		me.page.PageCssClasses = App.Proj.Site.Gen.ClsChapter + "n"
		if me.lang == App.Proj.Langs[0] {
			me.page.HrefDirLtr = "./index.html"
			me.page.HrefDirRtl = "./index." + App.Proj.DirModes.Rtl.Name + ".html"
		} else {
			me.page.HrefDirLtr = "./index." + me.lang + ".html"
			me.page.HrefDirRtl = "./index." + App.Proj.DirModes.Rtl.Name + "." + me.lang + ".html"
		}
		me.prepHomePage()
		numFilesWritten += me.genPageExecAndWrite(homename, nil, totalSizeRec)

	} else {
		series := chapter.parentSeries
		// me.page.HrefHome += "#" + strings.ToLower(series.Name)
		chaptitlewords := strings.Split(hEsc(trim(locStr(chapter.Title, me.lang))), " ")
		for i, word := range chaptitlewords {
			if len(word) < 16 {
				chaptitlewords[i] = "<nobr>" + word + "</nobr>"
			}
		}
		homelink := me.namePage(nil, 0, 0, "", "", "", 0, false) + ".html#" + series.Name + "_" + chapter.Name
		me.page.PageTitle = "<a href='" + homelink + "'><span>" + hEsc(locStr(series.Title, me.lang)) + ":</span></a> " + strings.Join(chaptitlewords, " ")
		me.page.PageTitleTxt = hEsc(locStr(series.Title, me.lang)) + ": " + hEsc(locStr(chapter.Title, me.lang))
		var author string
		if chapter.author != nil {
			author = strings.Replace(
				strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", chapter.author.String(false, true), 1),
				"%YEAR%", sIf(chapter.Year == 0, "", ",&nbsp;"+itoa(chapter.Year)), 1)
		}
		desc := locStr(chapter.DescHtml, me.lang)
		if desc == "" && chapter.Year != 0 && chapter.StoryUrls.LinkHref != "" {
			desc = "<a target='_blank' rel='noreferrer' href='https://" + chapter.StoryUrls.LinkHref + "'><pre title='" + strings.Join(chapter.StoryUrls.Alt, "&#10;") + "'>" + sIf(chapter.StoryUrls.DisplayUrl != "", chapter.StoryUrls.DisplayUrl, chapter.StoryUrls.LinkHref) + "</pre></a>"
			skiptitle := chapter.TitleOrig == "" && (me.lang == App.Proj.Langs[0] || locStr(chapter.Title, App.Proj.Langs[0]) == locStr(chapter.Title, me.lang))
			desc = "Story: " + sIf(skiptitle, "", "&quot;"+sIf(chapter.TitleOrig != "", chapter.TitleOrig, locStr(chapter.Title, App.Proj.Langs[0]))+"&quot;, ") + desc
		}
		me.page.PageDesc = sIf(desc == "", locStr(series.DescHtml, me.lang), desc) + author
		me.page.PageDescTxt = hEsc(sIf(desc == "", locStr(series.DescHtml, me.lang), desc))
		for qidx, quali := range App.Proj.Qualis {
			if quali.ExcludeInSiteGen {
				continue
			}
			for _, viewmode := range viewModes {
				me.page.PageCssClasses = App.Proj.Site.Gen.ClsChapter + viewmode
				for _, svdt := range chapter.versions {
					qname, qsizes, allpanels := quali.Name, map[int]int64{}, me.prepSheetPage(qidx, viewmode, chapter, svdt, pageNr)
					me.page.QualList = ""
					for i, q := range App.Proj.Qualis {
						var totalimgsize int64
						for sv, maxpidx := range allpanels {
							for pidx := 0; pidx <= maxpidx; pidx++ {
								if bgfile := fileStat(".build/" + App.Proj.Site.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg.png"); bgfile != nil && me.bgCol {
									totalimgsize += bgfile.Size()
								}
								name := me.namePanelPic(sv, pidx, q.SizeHint)
								if fileinfo := fileStat(strings.ToLower(".build/" + App.Proj.Site.Gen.PicDirName + "/" + name + sIf(q.SizeHint == 0, ".svg", ".png"))); fileinfo != nil {
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
						if q.ExcludeInSiteGen {
							continue
						}
						me.page.QualList += "<option value='" + me.namePage(chapter, q.SizeHint, pageNr, viewmode, "", me.lang, svdt, me.bgCol) + "'"
						if q.Name == qname {
							me.page.QualList += " selected='selected'"
						}
						me.page.QualList += ">" + q.Name + " (" + strSize(uint64(qsizes[i])) + ")</option>"
					}
					me.page.QualList = "<select disabled='disabled' title='" + hEsc(me.textStr("QualityHint")) + "' name='" + App.Proj.Site.Gen.IdQualiList + "' id='" + App.Proj.Site.Gen.IdQualiList + "'>" + me.page.QualList + "</select>"
					me.page.HrefDirLtr = "./" + me.namePage(chapter, quali.SizeHint, pageNr, viewmode, App.Proj.DirModes.Ltr.Name, me.lang, svdt, me.bgCol) + ".html"
					me.page.HrefDirRtl = "./" + me.namePage(chapter, quali.SizeHint, pageNr, viewmode, App.Proj.DirModes.Rtl.Name, me.lang, svdt, me.bgCol) + ".html"
					me.page.PageTitleTxt += " (" + itoa(pageNr) + "/" + itoa(len(chapter.SheetsPerPage)) + ")"
					pagename := me.namePage(chapter, quali.SizeHint, pageNr, viewmode, "", me.lang, svdt, me.bgCol)
					numFilesWritten += me.genPageExecAndWrite(pagename, chapter, totalSizeRec)
					if chapter.UrlJumpName != "" && viewmode == viewModes[0] && qidx == 1 &&
						pageNr <= 1 && (me.bgCol || !chapter.HasBgCol()) && !me.dirRtl {
						fileLinkOrCopy(".build/"+pagename+".html", ".build/"+chapter.UrlJumpName+sIf(me.lang == App.Proj.Langs[0], "", "."+me.lang)+".html")
						numFilesWritten++
					}
				}
			}
		}
	}
	return
}

const noice = true

func (me *siteGen) prepHomePage() {
	s := "<div class='" + App.Proj.Site.Gen.ClsNonViewerPage + "'>"
	for seryear := time.Now().Year(); seryear >= 2021; seryear-- {
		for _, series := range me.series {
			if !series.scanYearHas(seryear, true) {
				continue
			}
			var gotsheets bool
			for _, chapter := range series.Chapters {
				if chapter.scanYearLatest() != seryear {
					continue
				}
				if gotsheets = (len(chapter.sheets) > 0 && !chapter.Priv); gotsheets {
					break
				}
			}
			if series.Priv || len(series.Chapters) == 0 || !gotsheets {
				continue
			}

			var author string
			if series.author != nil {
				author = strings.Replace(
					strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", series.author.String(true, true), 1),
					"%YEAR%", sIf(series.Year == 0, "", ", "+itoa(series.Year)), 1)
			}
			s += "<span class='" + App.Proj.Site.Gen.ClsSeries + "'>"
			s += "<h5 id='" + strings.ToLower(series.Name) + "_" + itoa(seryear) + "' class='" + App.Proj.Site.Gen.ClsSeries + "'>" + hEsc(locStr(series.Title, me.lang)) + " (" + itoa(seryear) + ")</h5>"
			s += "<div class='" + App.Proj.Site.Gen.ClsSeries + "'>" + locStr(series.DescHtml, me.lang) + author + "</div>"
			s += "<span>"
			for _, chapter := range series.Chapters {
				if chapter.Priv || len(chapter.sheets) == 0 || chapter.scanYearLatest() != seryear {
					continue
				}
				numpages := len(chapter.SheetsPerPage)
				dt1, dt2 := chapter.dateRangeOfSheets(false)
				sdt1, sdt2 := dt1.Format("Jan 2006"), dt2.Format("Jan 2006")
				sdt := sdt1 + " - " + sdt2
				if sdt1 == sdt2 {
					sdt = dt1.Format("January 2006")
					if m := dt1.Month().String(); me.lang != App.Proj.Langs[0] {
						sdt = strings.Replace(sdt, m, me.textStr("Month_"+m), 1)
					}
				}
				chapmins := chapter.readDurationMinutes()
				title := strings.NewReplacer(
					"%MINS%", itoa(chapmins)+"-"+itoa(1+chapmins),
					"%NUMPGS%", itoa(numpages),
					"%NUMPNL%", itoa(chapter.NumPanels()),
					"%NUMSCN%", itoa(chapter.NumScans()),
					"%DATEINFO%", sdt,
				).Replace(me.textStr("ChapStats"))
				if numpages <= 1 {
					title = trim(title[1+strings.IndexByte(title, '/'):])
				}
				if App.Proj.percentTranslated(me.lang, series, chapter, nil, -1) < 50 {
					title += " " + me.textStr("Untransl")
				}
				picidxsheet, picidxpanel, picbgpos := 0.0, 0.0, ""
				if chapter.Pic != nil && len(chapter.Pic) >= 2 {
					picidxsheet, _ = chapter.Pic[0].(float64)
					picidxpanel, _ = chapter.Pic[1].(float64)
					if len(chapter.Pic) > 2 {
						picbgpos = chapter.Pic[2].(string)
					}
				}
				picname, chid := me.namePanelPic(chapter.sheets[int(picidxsheet)].versions[0], int(picidxpanel), App.Proj.Qualis[App.Proj.maxQualiIdx(true)].SizeHint), chapter.parentSeries.Name+"_"+chapter.Name
				s += "<a name='" + chid + "' id='" + chid + "' class='" + App.Proj.Site.Gen.ClsChapter + "' title='" + hEsc(title) + "' href='./" + me.namePage(chapter, App.Proj.Qualis[App.Proj.defaultQualiIdx].SizeHint, 1, "s", "", me.lang, 0, true) + ".html' style='background-image: url(\"" + sIf(os.Getenv("NOPICS") != "", "files/white.png", App.Proj.Site.Gen.PicDirName+"/"+picname) + ".png\"); " + sIf(picbgpos == "", "", "background-position: "+picbgpos) + "'>"
				s += "<div>" + hEsc(locStr(chapter.Title, me.lang)) + "</div>"
				s += "<span><span>" + itoa(chapmins) + "-" + itoa(1+chapmins) + me.textStr("Mins") + "</span><span>" +
					sIf(chapter.Year == 0, "&nbsp;", "&copy;"+itoa(chapter.Year)) + "&nbsp;" + chapter.author.String(true, true) +
					"</span></span>"
				s += "</a>"
			}
			s += "</span></span>"
		}
	}
	s += "<h5 id='books' class='" + App.Proj.Site.Gen.ClsSeries + "'>Downloads</h5>"
	s += "<span><ul>"
	booklink := func(repo string, pref string, ext string) string {
		return App.Proj.Books.RepoPath.Prefix + repo + App.Proj.Books.RepoPath.Infix + sIf(pref == "", "printcover", pref+me.lang+`_`+sIf(me.dirRtl, "rtl", "ltr")) + ext
	}
	for _, bookpub := range App.Proj.Books.Pubs {
		s += "<li id='book_" + bookpub.RepoName + "'><b style='font-size: xx-large'>" + bookpub.Title + "</b><ul>"
		s += "</li>"
		s += `<li>Screen 4K &mdash; <a target="_blank" rel=“noopener noreferrer“ href="` + booklink(bookpub.RepoName, "screen_", ".pdf") + `">PDF</a>, <a target="_blank" rel=“noopener noreferrer“ href="` + booklink(bookpub.RepoName, "screen_", ".cbz") + `">CBZ</a></li>`
		s += `<li>Print ~1700dpi &mdash; <a target="_blank" rel=“noopener noreferrer“ href="` + booklink(bookpub.RepoName, "print_", ".pdf") + `">PDF</a>, <a target="_blank" rel=“noopener noreferrer“ href="` + booklink(bookpub.RepoName, "", ".pdf") + `">Cover</a></li>`
		s += "<li>" + me.textStr("BookContents")
		for _, series := range bookpub.Series {
			if ser := App.Proj.seriesByName(series); ser != nil {
				s += "&nbsp;<a href='#" + ser.Name + "_" + itoa(bookpub.Year) + "'>" + hEsc(locStr(ser.Title, me.lang)) + "</a>&nbsp;"
			}
		}
		s += "</ul></li>"
	}
	s += "</ul></span>"
	s += "</div>"
	me.page.PageContent = s
}

func (me *siteGen) prepSheetPage(qIdx int, viewMode string, chapter *Chapter, svDt int64, pageNr int) map[*SheetVer]int {
	isfirstpanelonpage, quali := true, App.Proj.Qualis[qIdx]
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
			for k, v := range App.Proj.Site.Texts[me.lang] {
				if strings.HasPrefix(k, "Month_") {
					text = strings.Replace(text, k[len("Month_"):], v, -1)
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
		text := me.textStr("Bg" + sIf(!bgcol, "Bw", sIf(chapter.PercentColorized() < 100.0, "ColP", "Col")))
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
		istoplist, numpages := (sheets == nil), len(chapter.SheetsPerPage)

		shownums := map[int]bool{1: true, numpages: true, pageNr: true}
		if numpages <= 4 || !istoplist {
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
		pglast, shidx, percc := -1, 0, App.Proj.percentTranslated(me.lang, nil, chapter, nil, -1)
		for pgnr := 1; pgnr <= len(chapter.SheetsPerPage); pgnr++ {
			did, name := (pgnr == pageNr), me.namePage(chapter, quali.SizeHint, pgnr, viewMode, "", me.lang, svDt, me.bgCol)
			if did {
				s += "<li><b><a href='./" + name + ".html'>" + itoa(pgnr) + "</a></b></li>"
			} else if did = shownums[pgnr]; did {
				if perc := App.Proj.percentTranslated(me.lang, nil, chapter, nil, pgnr); perc < 0.0 || perc >= 50 || percc <= 0.0 {
					s += "<li>"
				} else {
					s += "<li class='nolang' title='" + me.textStr("Untransl") + "'>"
				}
				s += "<a href='./" + name + ".html'>" + itoa(pgnr) + "</a></li>"
			}
			if did {
				pglast = pgnr
			} else if pglast == pgnr-1 {
				s += "<li class='" + App.Proj.Site.Gen.APaging + "s'><span>&hellip;&nbsp;</span></li>"
			}

			numsheets := chapter.SheetsPerPage[pgnr-1]
			if pgnr == pageNr && istoplist {
				sheets = append(sheets, chapter.sheets[shidx:shidx+numsheets]...)
			}
			shidx += numsheets
		}

		if s = sIf(numpages == 1, "", s); s != "" {
			var pg int
			if pg = pageNr - 1; pg < 1 {
				pg = 1
			}
			pvis, phref := "hidden", me.namePage(chapter, quali.SizeHint, pg, viewMode, "", me.lang, svDt, me.bgCol)
			if pg = pageNr + 1; pg > numpages {
				pg = numpages
			}
			nvis, nhref, nhome := "none", me.namePage(chapter, quali.SizeHint, pg, viewMode, "", me.lang, svDt, me.bgCol), false
			if pageNr > 1 /*&& istoplist*/ {
				pvis = "visible"
			}
			if pageNr < numpages {
				nvis = "inline-block"
			} else if chapter.parentSeries.numNonPrivChaptersWithSheets() > 1 {
				nvis, nhome, nhref = "inline-block", true, me.namePage(nil, 0, 0, "", "", "", 0, false)
			}
			phref, nhref = phref+".html", nhref+".html"
			if nhome {
				nhref += "#" + chapter.parentSeries.Name + "_" + chapter.Name
			}
			ulid := App.Proj.Site.Gen.APaging
			if !istoplist {
				ulid += "b"
			}
			s = "<ul id='" + ulid + "'>" +
				"<li><a style='visibility: " + pvis + "' href='./" + strings.ToLower(phref) + "'>&larr;</a></li>" +
				s +
				"<li><a style='display: " + nvis + "' href='./" + strings.ToLower(nhref) + "'>&rarr;</a></li>" +
				"</ul>"
		}
		return s
	}
	me.page.PagesList, me.page.PageContent = pageslist(), "<div class='"+App.Proj.Site.Gen.ClsViewerPage+"'>"

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
			me.page.ViewerList += "<a class='" + App.Proj.Site.Gen.ClsPanel + "l' href='" + me.page.HrefViewAlt + "'>&nbsp;</a>"
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
				s += "<div id='" + firstrow + App.Proj.Site.Gen.ClsPanel + "r" + sv.id + itoa(i) + "' class='" + App.Proj.Site.Gen.ClsPanelRow
				if firstrow = ""; istop && viewMode == "r" {
					s += " " + App.Proj.Site.Gen.ClsPanelRow + "t"
				} else if istop {
					s += "' onfocus='" + App.Proj.Site.Gen.ClsPanel + "f(this)' tabindex='0"
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
				s += "<div class='" + App.Proj.Site.Gen.ClsPanelCol + "'"
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
			imgfilename := me.namePanelPic(sv, pidx, App.Proj.Qualis[0].SizeHint) + ".png"
			imgfilenamelo := imgfilename
			for i := qIdx; i > 0; i-- {
				filename := me.namePanelPic(sv, pidx, App.Proj.Qualis[i].SizeHint) + sIf(App.Proj.Qualis[i].SizeHint == 0, ".svg", ".png")
				if fileinfo := fileStat(".build/" + App.Proj.Site.Gen.PicDirName + "/" + filename); fileinfo != nil && fileinfo.Size() > 0 {
					imgfilename = filename
					break
				}
			}

			s += "<div id='" + firstpanel + App.Proj.Site.Gen.ClsPanel + "p" + sv.id + itoa(pidx) + "' class='" + App.Proj.Site.Gen.ClsPanel + "'"
			if firstpanel = ""; viewMode == "r" {
				s += " tabindex='0' onfocus='" + App.Proj.Site.Gen.ClsPanel + "f(this)'"
			}
			s += ">" + sv.genTextSvgForPanel(pidx, panel, me.lang, true, false)
			me.sheetPgNrs[sv] = pageNr
			s += "<img src='./" + sIf(os.Getenv("NOPICS") != "", "files/white.png", App.Proj.Site.Gen.PicDirName+"/"+imgfilename) + "'"
			if imgfilenamelo != imgfilename {
				s += " lowsrc='./" + App.Proj.Site.Gen.PicDirName + "/" + imgfilenamelo + "'"
			}
			if isfirstpanelonpage {
				isfirstpanelonpage = false
				s += " fetchpriority='high'"
			}
			if me.bgCol && sv.data.hasBgCol {
				if bgsvg := fileStat(".build/" + App.Proj.Site.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg.png"); bgsvg != nil {
					s += " style='background-image:url(\"./" + App.Proj.Site.Gen.PicDirName + "/" + sv.DtStr() + sv.id + itoa(pidx) + "bg.png\");'"
				}
			}
			s += "/>"
			s += "</div>"
			pidx++
		}
		return
	}
	cls := App.Proj.Site.Gen.ClsSheetsView
	if viewMode == "r" {
		cls = App.Proj.Site.Gen.ClsRowsView
	}
	me.page.PageContent += "<div class='" + App.Proj.Site.Gen.ClsViewer + " " + cls + "'>"
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
		_ = sheetver.ensurePrep(false, false)
		pidx = 0
		if viewMode != "r" {
			me.page.PageContent += "<div id='" + sheetver.id + "' class='" + App.Proj.Site.Gen.ClsSheet + "'>"
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
	if mzs := chapter.GenPanelSvgText.MozScale; mzs > 0.01 {
		me.page.PageContent = `<style type="text/css">
				symbol > svg.mz { -moz-transform:scale(` + ftoa(mzs, 2) + `) !important; }
			</style>` + me.page.PageContent
	}

	return allpanels
}

func (me *siteGen) genPageExecAndWrite(name string, chapter *Chapter, totalSizeRec *uint64) (numFilesWritten int) {
	me.page.LangsList = ""
	for _, lang := range App.Proj.Langs {
		title, imgsrcpath := lang, strings.Replace(App.Proj.Site.Gen.ImgSrcLang, "%LANG%", lang, -1)
		if langname := App.Proj.textStr(lang, "LangName"); langname != "" {
			title = langname
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
			me.page.LangsList += "<a class='" + App.Proj.Site.Gen.ClsPanel + "l' href='./" + href + ".html'><img alt='" + hEsc(title) + "' title='" + hEsc(title) + "' src='" + imgsrcpath + "'/></a>"
			me.page.LangsList += "</div>"
		}
	}
	if me.page.PageTitleTxt == "" {
		me.page.PageTitleTxt = me.page.PageTitle
	}
	if me.dirRtl {
		me.page.HrefDirCur, me.page.HrefDirAlt = me.page.HrefDirRtl, me.page.HrefDirLtr
		me.page.DirCurTitle, me.page.DirAltTitle = locStr(App.Proj.DirModes.Rtl.Title, me.lang), locStr(App.Proj.DirModes.Ltr.Title, me.lang)
	} else {
		me.page.HrefDirCur, me.page.HrefDirAlt = me.page.HrefDirLtr, me.page.HrefDirRtl
		me.page.DirCurTitle, me.page.DirAltTitle = locStr(App.Proj.DirModes.Ltr.Title, me.lang), locStr(App.Proj.DirModes.Rtl.Title, me.lang)
	}

	buf := bytes.NewBuffer(nil)
	if err := me.tmpl.ExecuteTemplate(buf, "site.html", &me.page); err != nil {
		panic(err)
	}
	outfilepath := ".build/" + strings.ToLower(name) + ".html"
	*totalSizeRec = *totalSizeRec + uint64(buf.Len())
	fileWrite(outfilepath, buf.Bytes())
	numFilesWritten++
	return
}

func (me *siteGen) textStr(key string) string {
	return App.Proj.textStr(me.lang, key)
}

func (me *siteGen) genSvgTextsFile(chapter *Chapter) string {
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
				xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">`
	for _, sheet := range chapter.sheets {
		for _, sv := range sheet.versions {
			pidx := 0
			sv.data.PanelsTree.iter(func(pnl *ImgPanel) {
				for i, area := range sv.panelAreas(pidx) {
					svg += "<symbol id=\"" + sv.id + "_" + itoa(pidx) + "t" + itoa(i+1) + "\">\t" +
						sv.genTextSvgForPanelArea(pidx, i, &area, me.lang, false, false) + "</symbol>"
				}
				pidx++
			})
		}
	}
	svg += `</svg>`
	fileWrite(".build/t."+chapter.parentSeries.Name+"."+chapter.Name+"."+me.lang+".svg", []byte(svg))
	return svg
}

func (me *siteGen) genAtomXml(totalSizeRec *uint64) (numFilesWritten int) {
	af, tlatest := App.Proj.Site.Feed, ""
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
			if series.Priv {
				continue
			}
			for _, chapter := range series.Chapters {
				if chapter.Priv {
					continue
				}
				pgnr, numpanels, numsheets, pages := -1, 0, 0, map[int]bool{}
				for _, sheet := range chapter.sheets {
					for _, sv := range sheet.versions {
						if dtstr := time.Unix(0, sv.dateTimeUnixNano).Format("2006-01-02"); dtstr >= nextolderdate && dtstr < pubdate {
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
					xml := `<entry><updated>` + tpub + `Z</updated>`
					xml += `<title>` + xEsc(locStr(chapter.parentSeries.Title, me.lang)) + `: ` + xEsc(locStr(chapter.Title, me.lang)) + `</title>`
					xml += "<id>info:" + contentHashStr([]byte(series.Name+"_"+chapter.Name+"_"+pubdate+"_"+"_"+me.lang)) + "</id>"
					pgname := me.namePage(chapter, App.Proj.Qualis[App.Proj.defaultQualiIdx].SizeHint, pgnr, "s", "", me.lang, 0, true)
					if chapter.UrlJumpName != "" && pgnr == 1 {
						pgname = chapter.UrlJumpName + sIf(me.lang == App.Proj.Langs[0], "", "."+me.lang)
					}
					xml += `<link href="http://` + App.Proj.Site.Host + "/" + pgname + `.html"/>`
					xml += `<author><name>` + App.Proj.Site.Host + `</name></author>`
					xml += `<content type="text">` + strings.NewReplacer(
						"%NUMSVS%", itoa(numsheets),
						"%NUMPNL%", itoa(numpanels),
						"%NUMPGS%", itoa(len(pages)),
					).Replace(locStr(af.ContentTxt, me.lang)) + `</content>`
					xmls = append(xmls, xml+`</entry>`)
				}
			}
		}
	}
	for _, bookpub := range App.Proj.Books.Pubs {
		xml := "<entry><updated>" + bookpub.PubDate + "T11:22:44Z</updated>"
		xml += `<title>Album: ` + xEsc(bookpub.Title) + `</title>`
		xml += "<id>info:" + contentHashStr([]byte(strings.Join(bookpub.Series, "+")+"_"+bookpub.RepoName+"_"+bookpub.PubDate+"_"+"_"+me.lang)) + "</id>"
		xml += `<link href="http://` + App.Proj.Site.Host + "/" + me.namePage(nil, 0, 0, "", "", "", 0, false) + `.html#book_` + bookpub.RepoName + `"/>`
		xml += `<author><name>` + App.Proj.Site.Host + `</name></author>`
		xml += `<content type="text">` + strings.NewReplacer(
			"%REPONAME%", bookpub.RepoName,
		).Replace(locStr(af.ContentTxtBook, me.lang)) + `</content>`
		xmls = append(xmls, xml+`</entry>`)
	}
	if len(xmls) > 0 && tlatest != "" {
		filename := af.Name + "." + me.lang + ".atom"
		s := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom" xml:lang="` + me.lang + `">`
		s += `<updated>` + tlatest + `Z</updated><title>` + hEsc(App.Proj.Site.Title) + `</title><link href="http://` + App.Proj.Site.Host + `"/><link rel="self" href="http://` + App.Proj.Site.Host + `/` + filename + `"/><id>http://` + App.Proj.Site.Host + "/</id>"
		s += "\n" + strings.Join(xmls, "\n") + "\n</feed>"
		*totalSizeRec = *totalSizeRec + uint64(len(s))
		fileWrite(".build/"+af.Name+"."+me.lang+".atom", []byte(s))
		numFilesWritten++
	}
	return
}

func (siteGen) namePanelPic(sheetVer *SheetVer, pIdx int, qualiSizeHint int) string {
	return sheetVer.DtStr() + sheetVer.id + itoa(pIdx) + "." + itoa(qualiSizeHint)
}

func (me *siteGen) namePage(chapter *Chapter, qualiSizeHint int, pageNr int, viewMode string, dirMode string, langId string, svDt int64, bgCol bool) string {
	if pageNr < 1 {
		pageNr = 1
	}
	if langId == "" {
		langId = me.lang
	}
	if dirMode == "" {
		if dirMode = App.Proj.DirModes.Ltr.Name; me.dirRtl {
			dirMode = App.Proj.DirModes.Rtl.Name
		}
	}
	if chapter == nil {
		return "index" + sIf(dirMode == App.Proj.DirModes.Ltr.Name, "", "."+App.Proj.DirModes.Rtl.Name) + sIf(langId == App.Proj.Langs[0], "", ".de")
	}
	return strings.ToLower(chapter.parentSeries.UrlName + "-" + chapter.UrlName + "-" + itoa(pageNr) + sIf(bgCol && chapter.HasBgCol(), "col", "bw") + strconv.FormatInt(svDt, 36) + viewMode + itoa(qualiSizeHint) + "-" + dirMode + "." + langId)
}
