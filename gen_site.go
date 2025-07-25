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

var viewModes = []string{"s"}

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
	dummy      bool
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
	PageDescTitle  string
	PageLang       string
	PageCssClasses string
	PageDirCur     string
	PageDirAlt     string
	DirCurTitle    string
	DirAltTitle    string
	LangsList      string
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
	SvgTextIdent   string
}

func (me siteGen) genSite(fromGui bool, _ map[string]bool) {
	var err error
	tstart := time.Now()
	me.series, me.sheetPgNrs, me.dummy = App.Proj.Series, map[*SheetVer]int{}, (os.Getenv("DUMMY") != "")
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
		numfilescopied := me.copyStaticFiles("")
		return "for " + itoa(numfilescopied) + " files"
	})

	timedLogged("SiteGen: generating (but mostly copying pre-generated) PNGs & SVGs...", func() string {
		chapterqstats := map[*Chapter]map[string][]int64{}
		for _, series := range App.Proj.Series {
			for _, chapter := range series.Chapters {
				if !(chapter.Priv || series.Priv) {
					chapterqstats[chapter] = map[string][]int64{}
				}
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
					for i := qidx; i >= 0 && size == 0; i-- {
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
			for _, me.dirRtl = range []bool{false, true /*KEEP this order of bools*/} {
				if me.dirRtl && App.Proj.DirModes.Rtl.Disabled {
					continue
				}
				me.bgCol = false
				numfileswritten += me.genPages(nil, 0, &totalsize)
				for _, me.bgCol = range []bool{false, true} {
					for _, series := range me.series {
						if series.Priv {
							continue
						}
						for _, chapter := range series.Chapters {
							if chapter.Priv {
								continue
							}
							if (me.bgCol && !chapter.hasBgCol()) || !chapter.isTransl(me.lang) {
								continue
							}
							for i := range chapter.SheetsPerPage {
								numfileswritten += me.genPages(chapter, 1+i, &totalsize)
							}
						}
					}
				}
				if !me.dirRtl {
					if App.Proj.Site.Feed.Name != "" {
						numfileswritten += me.genAtomXml(&totalsize)
					}
					for _, series := range me.series {
						for _, chapter := range series.Chapters {
							if chapter.isTransl(me.lang) {
								numfileswritten++
								totalsize += uint64(len(me.genSvgTextsFile(chapter)))
							}
						}
					}
				}
				if os.Getenv("NORTL") != "" || os.Getenv("NODIR") != "" {
					break
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
		m := make(map[string]*PanelSvgTextGen, len(App.Proj.Sheets.Panel.SvgText))
		for k, v := range App.Proj.Sheets.Panel.SvgText {
			m[k] = v
		}
		for _, series := range App.Proj.Series {
			if series.GenPanelSvgText != nil && series.GenPanelSvgText.cssName != "" && m[series.GenPanelSvgText.cssName] == nil {
				m[series.GenPanelSvgText.cssName] = series.GenPanelSvgText
			}
			for _, chap := range series.Chapters {
				if chap.GenPanelSvgText != nil && chap.GenPanelSvgText.cssName != "" && m[chap.GenPanelSvgText.cssName] == nil {
					m[chap.GenPanelSvgText.cssName] = chap.GenPanelSvgText
				}
			}
		}
		for k, svgtxt := range m {
			data, dstpath := []byte(App.Proj.cssFontFaces(nil)), filepath.Join(".build", relDirPath, "site_"+k+".css")
			for csssel, csslines := range svgtxt.Css {
				if csssel != "" {
					if csslines != nil && len(csslines) == 0 {
						csslines = svgtxt.Css[""]
					}
					css := csssel + "{\n"
					for k, v := range csslines {
						css += k + ":" + v + ";\n"
					}
					css += "}\n"
					data = append(data, css...)
				}
			}
			fileWrite(dstpath, data)
			numFilesWritten++
		}
		for _, fileinfo := range fileinfos {
			fn := fileinfo.Name()
			relpath := filepath.Join(relDirPath, fn)
			dstpath := filepath.Join(".build", relpath)
			if fileinfo.IsDir() {
				mkDir(dstpath)
				numFilesWritten += me.copyStaticFiles(relpath)
			} else if fn != siteTmplFileName {
				data := fileRead(filepath.Join(srcdirpath, fn))
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
				if !(chapter.Priv || series.Priv) {
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
				sv := sheet.versions[0]
				if pngfilepath := filepath.Join(sv.Data.DirPath, "strip.1.png"); fileStat(pngfilepath) != nil {
					if year, num, ok := strings.Cut(sv.parentSheet.name, "."); ok {
						splits := strings.Split(num, "_")
						for i, num := range splits {
							outfilepath := ".build/" + App.Proj.Site.Gen.PicDirName + "/" + sv.parentSheet.parentChapter.parentSeries.Name + "-" + year + "-" + num + ".png"
							fileLinkOrCopy(strings.ReplaceAll(pngfilepath, ".1.", "."+itoa(i+1)+"."), outfilepath)
							atomic.AddUint32(&numPngs, 1)
						}
					}
				}
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
	sv.Data.PanelsTree.each(func(panel *ImgPanel) {
		work.Add(1)
		numPanels++
		go func(pidx int) {
			for qidx, quali := range App.Proj.Qualis {
				if quali.ExcludeInSiteGen {
					continue
				}
				fext := sIf(quali.SizeHint == 0, ".svg", ".png")
				srcpath := filepath.Join(sv.Data.PicDirPath(quali.SizeHint), itoa(pidx)+fext)
				if fileinfo := fileStat(srcpath); fileinfo == nil && quali.SizeHint != 0 {
					break
				} else {
					for fs, again := uint32(fileinfo.Size()), true; again; {
						max := atomic.LoadUint32(&me.maxPicSize)
						again = (fs > max) && !atomic.CompareAndSwapUint32(&me.maxPicSize, max, fs)
					}
					atomic.AddUint64(&totalSize, uint64(fileinfo.Size()))
					dstpath := filepath.Join(".build", App.Proj.Site.Gen.PicDirName, me.namePanelPic(sv, pidx, quali.SizeHint)+fext)
					fileLinkOrCopy(srcpath, dstpath)
					if me.onPicSize != nil {
						me.onPicSize(sv.parentSheet.parentChapter, sv.ID+itoa(pidx), qidx, fileinfo.Size())
					}
					if quali.SizeHint == 0 {
						atomic.AddUint32(&numSvgs, 1)
					} else {
						atomic.AddUint32(&numPngs, 1)
					}
				}
			}
			if srcpath := filepath.Join(sv.Data.DirPath, "bg."+ftoa(App.Proj.Sheets.Panel.BgScale, 2)+"."+itoa(pidx)+".png"); sv.Data.hasBgCol != "" {
				if fileinfo := fileStat(srcpath); fileinfo != nil {
					atomic.AddUint64(&totalSize, uint64(fileinfo.Size()))
					dstpath := filepath.Join(".build", App.Proj.Site.Gen.PicDirName, sv.DtStr()+sv.ID+itoa(pidx)+"bg.png")
					fileLinkOrCopy(srcpath, dstpath)
					atomic.AddUint32(&numPngs, 1)
				}
			}
			work.Done()
		}(pidx)
		pidx++
	})

	if homepicname := sv.homePicName(); homepicname != "" && fileStat(sv.Data.HomePic) != nil {
		fileLinkOrCopy(sv.Data.HomePic, filepath.Join(".build", App.Proj.Site.Gen.PicDirName, homepicname))
		atomic.AddUint32(&numPngs, 1)
	}
	work.Wait()
	return
}

func (me *siteGen) genPages(chapter *Chapter, pageNr int, totalSizeRec *uint64) (numFilesWritten int) {
	homename, repl := me.namePage(nil, 0, 0, "", "", "", 0, false), strings.NewReplacer(
		"%LANG"+me.lang+"%", itoa(int(App.Proj.percentTranslated(me.lang, nil, nil, nil, -1))),
	)
	me.page = PageGen{
		SiteTitle:  sIf(me.dummy, "Site", App.Proj.Site.Title),
		SiteHost:   sIf(me.dummy, " site.host", App.Proj.Site.Host),
		SiteDesc:   sIf(me.dummy, "Site description", repl.Replace(hEsc(locStr(App.Proj.Site.Desc, me.lang)))),
		PageLang:   me.lang,
		HrefFeed:   "./" + App.Proj.Site.Feed.Name + "." + me.lang + ".atom",
		PageDirCur: "ltr",
		PageDirAlt: "rtl",
	}
	me.page.SiteTitleEsc = hEsc(me.page.SiteTitle)
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
		me.page.PageTitle = "<span>" + sIf(me.dummy, "Page Title", (me.textStr("HomeTitle"))) + "</span>"
		me.page.PageTitleTxt = hEsc(me.textStr("HomeTitleTxt"))
		me.page.PageDesc = sIf(me.dummy, "Page Description", repl.Replace(hEsc(me.textStr("HomeDesc"))))
		me.page.PageDescTxt = me.page.PageDesc
		me.page.PageDescTitle = ` title="` + me.txtStats(App.Proj.numPages(true, me.lang), App.Proj.numPanels(true, me.lang), App.Proj.numSheets(true, me.lang), sIf(me.page.SiteTitle == "ducksfan", "2025-", "2021-")+itoa(App.Proj.scanYearLatest(true, me.lang)), nil) + `"`
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
		if chapter.GenPanelSvgText.cssName != "" {
			me.page.SvgTextIdent = chapter.GenPanelSvgText.cssName
		} else if series.GenPanelSvgText.cssName != "" {
			me.page.SvgTextIdent = series.GenPanelSvgText.cssName
		}
		// me.page.HrefHome += "#" + strings.ToLower(series.Name)
		chaptitlewords := strings.Split(hEsc(trim(locStr(chapter.Title, me.lang))), " ")
		for i, word := range chaptitlewords {
			if len(word) < 16 {
				chaptitlewords[i] = "<nobr>" + word + "</nobr>"
			}
		}
		homelink := me.namePage(nil, 0, 0, "", "", "", 0, false) + ".html#" + series.Name + "_" + chapter.Name
		me.page.PageTitle = "<a href='" + homelink + "'><span>" + hEsc(locStr(series.Title, me.lang)) + ":</span></a> <span>" + strings.Join(chaptitlewords, " ") + "</span>"
		me.page.PageTitleTxt = hEsc(locStr(series.Title, me.lang)) + ": " + hEsc(locStr(chapter.Title, me.lang))
		var author string
		if chapter.author != nil {
			author = strings.Replace(
				strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", chapter.author.str(false, true), 1),
				"%YEAR%", sIf(chapter.Year == 0, "", itoa(chapter.Year)), 1)
			me.page.PageTitleTxt += ", " + strings.ReplaceAll(author, "&nbsp;", " ")
		}
		if len(chapter.SheetsPerPage) > 1 {
			me.page.PageTitleTxt += " (" + itoa(pageNr) + "/" + itoa(len(chapter.SheetsPerPage)) + ")"
		}
		desc := locStr(chapter.DescHtml, me.lang)
		if desc == "" && chapter.Year != 0 && chapter.StoryUrls.LinkHref != "" {
			desc = "<a target='_blank' rel='noreferrer' href='https://" + chapter.StoryUrls.LinkHref + "'><pre title='" + strings.Join(chapter.StoryUrls.Alt, "&#10;") + "'>" + sIf(chapter.StoryUrls.DisplayUrl != "", chapter.StoryUrls.DisplayUrl, chapter.StoryUrls.LinkHref) + "</pre></a>"
			skiptitle := chapter.TitleOrig == "" && (me.lang == App.Proj.Langs[0] || locStr(chapter.Title, App.Proj.Langs[0]) == locStr(chapter.Title, me.lang))
			desc = "Story: " + sIf(skiptitle, "", "&quot;"+sIf(chapter.TitleOrig != "", chapter.TitleOrig, locStr(chapter.Title, App.Proj.Langs[0]))+"&quot;, ") + desc
		}
		desc = sIf(desc == "", locStr(series.DescHtml, me.lang), desc)
		me.page.PageDesc = desc + sIf(author == "", "", " ("+author+")")
		for _a, i := "</a>", strings.Index(desc, "<a "); i >= 0; i = strings.Index(desc, "<a ") {
			if i2 := strings.Index(desc, _a); i2 > i {
				desc = desc[:i] + desc[i2+len(_a):]
			} else {
				break
			}
		}
		me.page.PageDescTxt = desc + author
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
								if bgfile := fileStat(".build/" + App.Proj.Site.Gen.PicDirName + "/" + sv.DtStr() + sv.ID + itoa(pidx) + "bg.png"); bgfile != nil && me.bgCol {
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
					pagename := me.namePage(chapter, quali.SizeHint, pageNr, viewmode, "", me.lang, svdt, me.bgCol)
					numFilesWritten += me.genPageExecAndWrite(pagename, chapter, totalSizeRec)
					if chapter.UrlJumpName != "" && viewmode == viewModes[0] && qidx == 1 &&
						pageNr <= 1 && (me.bgCol || !chapter.hasBgCol()) && !me.dirRtl {
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
				if gotsheets = (chapter.scanYearLatest() == seryear) && (len(chapter.sheets) > 0) && (!chapter.Priv) && chapter.isTransl(me.lang); gotsheets {
					break
				}
			}
			if series.Priv || len(series.Chapters) == 0 || !gotsheets {
				continue
			}

			s += "<span class='" + App.Proj.Site.Gen.ClsSeries + "'>"

			s += "<h5 title='" + me.txtStats(series.numPages(true, me.lang), series.numPanels(true, me.lang), series.numSheets(true, me.lang), itoa(seryear), nil) + "' id='" + strings.ToLower(series.Name) + "_" + itoa(seryear) + "' class='" + App.Proj.Site.Gen.ClsSeries + "'>" + hEsc(sIf(me.dummy, "Series Title", locStr(series.Title, me.lang))) + " (" + itoa(seryear) + ")</h5>"
			s += "<div class='" + App.Proj.Site.Gen.ClsSeries + "'>" + sIf(me.dummy, "Series Description", locStr(series.DescHtml, me.lang)) + "</div>"
			s += "<span>"
			chaps := ""
			for _, chapter := range series.Chapters {
				if chapter.Priv || len(chapter.sheets) == 0 || chapter.scanYearLatest() != seryear || !chapter.isTransl(me.lang) {
					continue
				}
				numpages := len(chapter.SheetsPerPage)
				dt1, dt2 := chapter.dateRangeOfSheets(false, seryear)
				sdt1, sdt2 := dt1.Format("Jan 2006"), dt2.Format("Jan 2006")
				sdt := sdt1 + " - " + sdt2
				if sdt1 == sdt2 {
					sdt = dt1.Format("January 2006")
					if m := dt1.Month().String(); me.lang != App.Proj.Langs[0] {
						sdt = strings.Replace(sdt, m, me.textStr("Month_"+m), 1)
					}
				}
				title := me.txtStats(numpages, chapter.numPanels(), chapter.numScans(), sdt, chapter)
				if App.Proj.percentTranslated(me.lang, series, chapter, nil, -1) < 50 {
					title = me.textStr("Untransl") + " " + title
				}
				picidxsheet, picbgpos := 0.0, "center center"
				if chapter.HomePic != nil && len(chapter.HomePic) > 0 {
					if picidxsheet = chapter.HomePic[0].(float64); len(chapter.HomePic) > 2 {
						picbgpos = chapter.HomePic[2].(string)
					}
				}
				s, picname, chid := "", chapter.sheets[int(picidxsheet)].versions[0].homePicName(), chapter.parentSeries.Name+"_"+chapter.Name
				s += "<a name='" + chid + "' id='" + chid + "' class='" + App.Proj.Site.Gen.ClsChapter + "' title='" + hEsc(title) + "' href='./" + me.namePage(chapter, App.Proj.Qualis[App.Proj.defaultQualiIdx].SizeHint, 1, "s", "", me.lang, 0, true) + ".html' style='background-image: url(\"" + sIf(os.Getenv("NOPICS") != "", "files/white.png", App.Proj.Site.Gen.PicDirName+"/"+sIf(me.dummy, "nope.png", picname)) + "\"); " + sIf(picbgpos == "", "", "background-position: "+picbgpos) + "'>"
				s += "<h6>" + hEsc(sIf(me.dummy, "Chapter Title", locStr(chapter.Title, me.lang))) + "</h6>"
				chapmins := iIf(me.dummy, 1, chapter.readDurationMinutes())
				s += "<span><span>" + itoa(chapmins) + "-" + itoa(1+chapmins) + me.textStr("Mins") + "</span><span>" +
					sIf(chapter.Year == 0 || me.page.SiteTitle == "ducksfan", "&nbsp;", "&copy;"+itoa(iIf(me.dummy, 1234, chapter.Year))) + "&nbsp;" + sIf(me.dummy, "Author Name", chapter.author.str(true, true)) +
					"</span></span>"
				s += "</a>"
				chaps = chaps + s
			}
			s += chaps + "</span></span>"
		}
	}
	if false && !me.dummy {
		s += "<h5 id='books' class='" + App.Proj.Site.Gen.ClsSeries + "'>Downloads</h5>"
		if !me.dirRtl {
			s += "<div>(" + me.textStr("DownloadAlt")
			for _, tld := range []string{"lc", "li", "gs"} {
				s += `&nbsp;&mdash;&nbsp;<a target="_blank" rel="noopener noreferrer" href="https://libgen.` + tld + `/series.php?id=403594">.` + tld + "</a>"
			}
			s += ")</div>"
		}
		s += "<span><ul>"
		booklink := func(repo string, pref string, ext string) string {
			return App.Proj.Books.RepoPath.Prefix + repo + App.Proj.Books.RepoPath.Infix + bookFileName(repo, pref, me.lang, me.dirRtl, ext)
		}
		for _, bookpub := range App.Proj.Books.Pubs {
			s += "<li id='book_" + bookpub.RepoName + "'><b style='font-size: xx-large'>" + bookpub.Title + "</b><ul style='margin-bottom: 1em;'>"

			numpg := strings.Replace(me.textStr("ChapStats"), "%NUMPGS%", itoa(bookpub.NumPages.Screen), 1)
			numpg = strings.Replace(numpg[:strings.IndexByte(numpg, ')')], "(", "", 1)
			s += `<li>Screen 4K &mdash; <a target="_blank" rel="noopener noreferrer" href="` + booklink(bookpub.RepoName, "screen", ".pdf") + `">PDF</a>, <a target="_blank" rel="noopener noreferrer" href="` + booklink(bookpub.RepoName, "screen", ".cbz") + `">CBZ</a> &mdash; (` + me.textStr("OrientL") + `, ` + numpg + `)</li>`
			numpg = strings.Replace(me.textStr("ChapStats"), "%NUMPGS%", itoa(bookpub.NumPages.Print), 1)
			numpg = strings.Replace(numpg[:strings.IndexByte(numpg, ')')], "(", "", 1)
			s += `<li>Print &tilde;1700dpi &mdash; <a target="_blank" rel="noopener noreferrer" href="` + booklink(bookpub.RepoName, "print", ".pdf") + `">PDF</a>, <a target="_blank" rel="noopener noreferrer" href="` + booklink(bookpub.RepoName, "", ".pdf") + `">Cover</a> &mdash; (` + me.textStr("OrientP") + `, ` + numpg + `)</li>`

			s += "<li>" + me.textStr("BookContents")
			for _, series := range bookpub.Series {
				if ser := App.Proj.seriesByName(series); ser != nil {
					s += "&nbsp;<a href='#" + ser.Name + "_" + itoa(bookpub.Year) + "'>" + hEsc(locStr(ser.Title, me.lang)) + "</a>&nbsp;"
				}
			}
			s += "</li></ul></li>"
		}
		s += "</ul></span>"
	}
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
		if bgcol && !chapter.hasBgCol() {
			continue
		}
		text := me.textStr("Bg" + sIf(!bgcol, "Bw", sIf(chapter.percentColorized() < 100.0, "ColP", "Col")))
		if perc := chapter.percentColorized(); bgcol && perc < 100.0 {
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
				"<li><a style='visibility: " + pvis + "' href='./" + strings.ToLower(phref) + "'>&#9668;</a></li>" +
				s +
				"<li><a style='display: " + nvis + "' href='./" + strings.ToLower(nhref) + "'>&#9658;</a></li>" +
				"</ul>"
		}
		return s
	}
	me.page.PagesList, me.page.PageContent = pageslist(), "<div class='"+App.Proj.Site.Gen.ClsViewerPage+"'>"

	var iter func(*SheetVer, *ImgPanel, bool) string
	pidx, allpanels, firstpanel, firstrow := 0, map[*SheetVer]int{}, "f", "f"
	iter = func(sv *SheetVer, panel *ImgPanel, istop bool) (s string) {
		assert(len(panel.SubCols) == 0 || len(panel.SubRows) == 0)

		if len(panel.SubRows) > 0 {
			for i := range panel.SubRows {
				sr := &panel.SubRows[i]
				s += "<div id='" + firstrow + App.Proj.Site.Gen.ClsPanel + "r" + sv.ID + itoa(i) + "' class='" + App.Proj.Site.Gen.ClsPanelRow
				if firstrow = ""; istop {
					s += "' onfocus='" + App.Proj.Site.Gen.ClsPanel + "f(this)' tabindex='0"
				}
				s += "'>" + iter(sv, sr, false) + "</div>"
			}

		} else if len(panel.SubCols) > 0 {
			for i := range panel.SubCols {
				sc := &panel.SubCols[i]
				s += "<div class='" + App.Proj.Site.Gen.ClsPanelCol + "'"
				pw, sw := sc.Rect.Dx(), panel.Rect.Dx()
				pp := 100.0 / (float64(sw) / float64(pw))
				s += " style='width: " + ftoa(pp, 8) + "%'"
				s += ">" + iter(sv, sc, false) + "</div>"
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

			s += "<div id='" + firstpanel + App.Proj.Site.Gen.ClsPanel + "p" + sv.ID + itoa(pidx) + "' class='" + App.Proj.Site.Gen.ClsPanel + "'"
			firstpanel = ""
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
			if me.bgCol && sv.Data.hasBgCol != "" {
				if bgsvg := fileStat(".build/" + App.Proj.Site.Gen.PicDirName + "/" + sv.DtStr() + sv.ID + itoa(pidx) + "bg.png"); bgsvg != nil {
					s += " style='background-image:url(\"./" + App.Proj.Site.Gen.PicDirName + "/" + sv.DtStr() + sv.ID + itoa(pidx) + "bg.png\");'"
				}
			}
			s += "/>"
			s += "</div>"
			pidx++
		}
		return
	}
	cls := App.Proj.Site.Gen.ClsSheetsView
	me.page.PageContent += "<div class='" + App.Proj.Site.Gen.ClsViewer + " " + cls + "'>"
	for _, sheet := range sheets {
		sheetver := sheet.versions[0]
		if svDt > 0 {
			for i := range sheet.versions {
				if sheet.versions[i].DateTimeUnixNano >= svDt {
					sheetver = sheet.versions[i]
				}
			}
		}
		_ = sheetver.ensurePrep(false, false)
		pidx = 0
		me.page.PageContent += "<div id='" + sheetver.ID + "' class='" + App.Proj.Site.Gen.ClsSheet + "'>"
		me.page.PageContent += iter(sheetver, sheetver.Data.PanelsTree, true)
		me.page.PageContent += "</div>"
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
	var numlangs int
	for i, lang := range App.Proj.Langs {
		if i != 0 && chapter != nil && !chapter.isTransl(lang) {
			continue
		}
		title, imgsrcpath := lang, strings.Replace(App.Proj.Site.Gen.ImgSrcLang, "%LANG%", lang, -1)
		if langname := App.Proj.textStr(lang, "LangName"); langname != "" {
			title = langname
		}
		if numlangs++; lang == me.lang {
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
	if numlangs <= 1 {
		me.page.LangsList = "<span style='display: none;'>" + me.page.LangsList + "</span>"
	}
	if me.page.PageTitleTxt == "" {
		panic(me.page.PageTitle)
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
			sv.Data.PanelsTree.each(func(pnl *ImgPanel) {
				for i, area := range sv.panelAreas(pidx) {
					svg += "<symbol id=\"" + sv.ID + "_" + itoa(pidx) + "t" + itoa(i+1) + "\">\t" +
						sv.genTextSvgForPanelArea(pidx, i, &area, me.lang, false, false, area.PointTo != nil) + "</symbol>"
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
				if chapter.Priv || !chapter.isTransl(me.lang) {
					continue
				}
				pgnr, numpanels, numsheets, pages := -1, 0, 0, map[int]bool{}
				for _, sheet := range chapter.sheets {
					for _, sv := range sheet.versions {
						if dtstr := time.Unix(0, sv.DateTimeUnixNano).Format("2006-01-02"); dtstr >= nextolderdate && dtstr < pubdate {
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
					xml += `<link href="https://` + App.Proj.Site.Host + "/" + pgname + `.html"/>`
					xml += `<author><name>` + App.Proj.Site.Host + `</name></author>`
					xml += `<content type="text">`
					content := strings.NewReplacer(
						"%NUMSVS%", itoa(numsheets),
						"%NUMPNL%", itoa(numpanels),
						"%NUMPGS%", itoa(len(pages)),
					).Replace(locStr(af.ContentTxt, me.lang))
					xmls = append(xmls, xml+content+`</content></entry>`)
				}
			}
		}
	}
	if false {
		for _, bookpub := range App.Proj.Books.Pubs {
			xml := "<entry><updated>" + bookpub.PubDate + "T11:22:44Z</updated>"
			xml += `<title>Album: ` + xEsc(bookpub.Title) + `</title>`
			xml += "<id>info:" + contentHashStr([]byte(strings.Join(bookpub.Series, "+")+"_"+bookpub.RepoName+"_"+bookpub.PubDate+"_"+"_"+me.lang)) + "</id>"
			xml += `<link href="https://` + App.Proj.Site.Host + "/" + me.namePage(nil, 0, 0, "", "", "", 0, false) + `.html#book_` + bookpub.RepoName + `"/>`
			xml += `<author><name>` + App.Proj.Site.Host + `</name></author>`
			xml += `<content type="text">` + strings.NewReplacer(
				"%REPONAME%", bookpub.RepoName,
			).Replace(locStr(af.ContentTxtBook, me.lang)) + `</content>`
			xmls = append(xmls, xml+`</entry>`)
		}
	}
	if len(xmls) > 0 && tlatest != "" {
		filename := af.Name + "." + me.lang + ".atom"
		s := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom" xml:lang="` + me.lang + `">`
		s += `<updated>` + tlatest + `Z</updated><title>` + hEsc(App.Proj.Site.Title) + `</title><link href="https://` + App.Proj.Site.Host + `"/><link rel="self" href="https://` + App.Proj.Site.Host + `/` + filename + `"/><id>http://` + App.Proj.Site.Host + "/</id>"
		s += "\n" + strings.Join(xmls, "\n") + "\n</feed>"
		*totalSizeRec = *totalSizeRec + uint64(len(s))
		fileWrite(".build/"+af.Name+"."+me.lang+".atom", []byte(s))
		numFilesWritten++
	}
	return
}

func (me *siteGen) txtStats(numPg int, numPnl int, numScn int, dtStr string, chap *Chapter) string {
	s := strings.NewReplacer(
		"%NUMPGS%", itoa(numPg),
		"%NUMPNL%", itoa(numPnl),
		"%NUMSCN%", itoa(numScn),
		"%DATEINFO%", dtStr,
	).Replace(me.textStr("ChapStats"))
	if i1, i2 := strings.IndexByte(s, '('), strings.IndexByte(s, ')'); i1 > 0 && i2 > i1 {
		s = s[:i1] + sIf(numPg == 1, "", s[i1+1:i2]) + s[i2+1:]
	}
	if chap != nil && chap.author != nil {
		s += "\n(Story: ©" + sIf(chap.Year > 0, itoa(chap.Year), "") + " " + chap.author.str(false, false) + ")"
	}
	return s
}

func (siteGen) namePanelPic(sheetVer *SheetVer, pIdx int, qualiSizeHint int) string {
	return sheetVer.DtStr() + sheetVer.ID + itoa(pIdx) + "." + itoa(qualiSizeHint)
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
	return strings.ToLower(chapter.parentSeries.UrlName + "-" + chapter.UrlName + "-" + itoa(pageNr) + sIf(bgCol && chapter.hasBgCol(), "col", "bw") + strconv.FormatInt(svDt, 36) + viewMode + itoa(qualiSizeHint) + "-" + dirMode + "." + langId)
}
