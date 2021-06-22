package main

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	LangsList      string
	ViewerList     string
	QualList       string
	PagesList      string
	PageContent    string
	FooterHtml     string
	HrefHome       string
	FeedHref       string
}

func siteGenFully(map[string]bool) {
	siteGen{}.genSite(true)
}

func siteGenPagesOnly(map[string]bool) {
	siteGen{}.genSite(false)
}

type siteGen struct {
	tmpl *template.Template
	page PageGen
	lang string
}

func (me siteGen) genSite(fully bool) {
	var err error
	tstart := time.Now()
	printLn("SiteGen started. When done, result will open in new window.")
	defer func() {
		if err := recover(); err != nil {
			printLn("SiteGen Error: ", err)
		}
	}()

	if fully {
		rmDir(".build")
		mkDir(".build")
		mkDir(".build/img/")
	} else if fileinfos, err := os.ReadDir(".build"); err == nil {
		for _, finfo := range fileinfos {
			if !finfo.IsDir() {
				_ = os.Remove(".build/" + finfo.Name())
			}
		}
	} else if !os.IsNotExist(err) {
		panic(err)
	}

	timedLogged("SiteGen: copying non-markup files to .build...", func() string {
		numfileswritten, modifycssfiles := 0, App.Proj.Gen.PanelSvgText.AppendToFiles
		if modifycssfiles == nil {
			modifycssfiles = map[string]bool{}
		}
		if fileinfos, err := os.ReadDir("sitetmpl"); err != nil {
			panic(err)
		} else {
			for _, fileinfo := range fileinfos {
				if fn := fileinfo.Name(); !(fileinfo.IsDir() || strings.Contains(strings.ToLower(filepath.Ext(fn)), "htm")) {
					if data, err := os.ReadFile("sitetmpl/" + fn); err != nil {
						panic(err)
					} else {
						if modifycssfiles[fn] {
							for csssel, csslines := range App.Proj.Gen.PanelSvgText.Css {
								if csssel != "" {
									if csslines == nil {
										csslines = App.Proj.Gen.PanelSvgText.Css[""]
									}
									data = append([]byte(csssel+"{"+strings.Join(csslines, ";")+"}\n"), data...)
								}
							}
						}
						if err := os.WriteFile(".build/"+fn, data, os.ModePerm); err != nil {
							panic(err)
						}
						numfileswritten++
					}
				}
			}
		}
		return "for " + strconv.Itoa(numfileswritten) + " file(s)"
	})

	if fully {
		timedLogged("SiteGen: generating PNGs...", func() string {
			numpngs, numsheets, numpanels := me.genPngs()
			numpngs += me.genThumbsPngs()
			return "to generate " + itoa(numpngs) + " PNG(s) for " + itoa(numpanels) + " panel(s) from " + itoa(numsheets) + " sheet(s)"
		})
	}

	timedLogged("SiteGen: generating markup files...", func() string {
		numfileswritten := 0
		me.tmpl, err = template.New("foo").ParseFiles("sitetmpl/_tmpl.html")
		if err != nil {
			panic(err)
		}
		for _, me.lang = range App.Proj.Langs {
			numfileswritten += me.genPages(nil, nil, 0)
			for _, series := range App.Proj.Series {
				for _, chapter := range series.Chapters {
					if chapter.SheetsPerPage > 0 {
						for i := 1; i <= (len(chapter.sheets) / chapter.SheetsPerPage); i++ {
							numfileswritten += me.genPages(series, chapter, i)
						}
					} else {
						numfileswritten += me.genPages(series, chapter, 0)
					}
				}
			}
			if App.Proj.AtomFile.Name != "" {
				numfileswritten += me.genAtomXml()
			}
		}
		return "for " + strconv.Itoa(numfileswritten) + " file(s)"
	})

	printLn("SiteGen: DONE after " + time.Now().Sub(tstart).String())
	browserCmd[len(browserCmd)-1] = "--app=file://" + os.Getenv("PWD") + "/.build/index.html"
	cmd := exec.Command(browserCmd[0], browserCmd[1:]...)
	if err := cmd.Run(); err != nil {
		printLn(err)
	}
}

func (*siteGen) genPngs() (numPngs int, numSheets int, numPanels int) {
	var numpngs atomic.Value
	numpngs.Store(0)
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			for _, sheet := range chapter.sheets {
				for _, sheetver := range sheet.versions {
					numSheets++
					sheetver.ensurePrep(false, false)
					srcimgfile, err := os.Open(sheetver.data.bwFilePath)
					if err != nil {
						panic(err)
					}
					imgsrc, _, err := image.Decode(srcimgfile)
					if err != nil {
						panic(err)
					}
					_ = srcimgfile.Close()

					var pidx int
					var work sync.WaitGroup
					contenthash := App.Proj.data.ContentHashes[sheetver.fileName]
					sheetver.data.PanelsTree.iter(func(panel *ImgPanel) {
						work.Add(1)
						numPanels++
						go func(pidx int) {
							for _, quali := range App.Proj.Qualis {
								name := strings.ToLower(contenthash + itoa(quali.SizeHint) + itoa(pidx))
								pw, ph, sw := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y, sheetver.data.PanelsTree.Rect.Max.X-sheetver.data.PanelsTree.Rect.Min.X
								width := float64(quali.SizeHint) / (float64(sw) / float64(pw))
								height := width / (float64(pw) / float64(ph))
								w, h := int(width), int(height)
								var wassamesize bool
								writeFile(".build/img/"+name+".png", imgSubRectPng(imgsrc.(*image.Gray), panel.Rect, &w, &h, quali.SizeHint/640, 0, false, &wassamesize))
								numpngs.Store(1 + numpngs.Load().(int))
								if wassamesize {
									break
								}
							}
							work.Done()
						}(pidx)
						pidx++
					})
					work.Wait()
				}
			}
		}
	}
	numPngs = numpngs.Load().(int)
	return
}

func (me *siteGen) genPages(series *Series, chapter *Chapter, pageNr int) (numFilesWritten int) {
	assert((series == nil) == (chapter == nil))
	name := "index"
	me.page = PageGen{
		SiteTitle:  hEsc(App.Proj.Title),
		SiteDesc:   hEsc(locStr(App.Proj.Desc, me.lang)),
		PageLang:   me.lang,
		FooterHtml: me.textStr("FooterHtml"),
		FeedHref:   "./" + App.Proj.AtomFile.Name + "." + me.lang + ".atom",
	}
	if me.lang != App.Proj.Langs[0] {
		name += "." + me.lang
	}
	me.page.HrefHome = "./" + name + ".html"

	if series == nil && chapter == nil {
		me.page.PageTitle = hEsc(me.textStr("HomeTitle"))
		me.page.PageDesc = hEsc(me.textStr("HomeDesc"))
		me.prepHomePage()
		numFilesWritten += me.genPageExecAndWrite(name)
	} else {
		me.page.PageCssClasses = "chapter"
		me.page.HrefHome += "#" + strings.ToLower(series.Name)
		me.page.PageTitle = "<span>" + hEsc(locStr(series.Title, me.lang)) + ":</span> " + hEsc(locStr(chapter.Title, me.lang))
		me.page.PageTitleTxt = hEsc(locStr(series.Title, me.lang)) + ": " + hEsc(locStr(chapter.Title, me.lang))
		var authorinfo string
		if series.Author != "" {
			authorinfo = strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", series.Author, 1)
		}
		me.page.PageDesc = hEsc(locStr(series.Desc, me.lang)) + authorinfo
		for _, viewmode := range viewModes {
			for qidx, quali := range App.Proj.Qualis {
				name = me.namePage(series, chapter, quali.SizeHint, pageNr, viewmode, me.lang)

				allpanels := me.prepSheetPage(qidx, viewmode, series, chapter, pageNr)
				me.page.QualList = ""
				var prevtotalimgsize int64
				for i, q := range App.Proj.Qualis {
					var totalimgsize int64
					for contenthash, maxpidx := range allpanels {
						for pidx := 0; pidx <= maxpidx; pidx++ {
							name := strings.ToLower(contenthash + itoa(q.SizeHint) + itoa(pidx))
							if fileinfo, err := os.Stat(strings.ToLower(".build/img/" + name + ".png")); err == nil {
								totalimgsize += fileinfo.Size()
							}
						}
					}
					if i != 0 && totalimgsize <= prevtotalimgsize {
						break
					}
					prevtotalimgsize = totalimgsize

					me.page.QualList += "<option value='" + me.namePage(series, chapter, q.SizeHint, pageNr, viewmode, me.lang) + "'"
					if q.Name == quali.Name {
						me.page.QualList += " selected='selected'"
					}
					imgsizeinfo := itoa(int(totalimgsize/1024)) + "KB"
					if mb := totalimgsize / 1048576; mb > 0 {
						imgsizeinfo = strconv.FormatFloat(float64(totalimgsize)/1048576.0, 1, 'f', 64) + "MB"
					}
					me.page.QualList += ">" + q.Name + " (" + imgsizeinfo + ")" + "</option>"
				}
				me.page.QualList = "<select disabled='disabled' title='" + hEsc(me.textStr("QualityHint")) + "' name='" + App.Proj.Gen.IdQualiList + "' id='" + App.Proj.Gen.IdQualiList + "'>" + me.page.QualList + "</select>"

				numFilesWritten += me.genPageExecAndWrite(name)
			}
		}
	}
	return
}

func (me *siteGen) prepHomePage() {
	me.page.PageContent = "<div class='" + App.Proj.Gen.ClsNonViewerPage + "'>"
	for _, series := range App.Proj.Series {
		var authorinfo string
		if series.Author != "" {
			authorinfo = strings.Replace(me.textStr("TmplAuthorInfoHtml"), "%AUTHOR%", series.Author, 1)
		}
		me.page.PageContent += "<span class='" + App.Proj.Gen.ClsSeries + "' style='background-image: url(\"./img/s" + itoa(App.Proj.NumSheetsInHomeBgs) + strings.ToLower(series.Name) + ".png\");'><span><h5 id='" + strings.ToLower(series.Name) + "' class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(locStr(series.Title, me.lang)) + "</h5><div class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(locStr(series.Desc, me.lang)) + authorinfo + "</div>"
		me.page.PageContent += "<ul class='" + App.Proj.Gen.ClsSeries + "'>"
		for _, chapter := range series.Chapters {
			me.page.PageContent += "<li class='" + App.Proj.Gen.ClsChapter + "'><a href='./" + me.namePage(series, chapter, App.Proj.Qualis[0].SizeHint, 1, "s", me.lang) + ".html'>" + hEsc(locStr(chapter.Title, me.lang)) + "</a></li>"
		}
		me.page.PageContent += "</ul></span><div></div></span>"
	}
	me.page.PageContent += "</div>"
}

func (me *siteGen) prepSheetPage(qIdx int, viewMode string, series *Series, chapter *Chapter, pageNr int) map[string]int {
	quali := App.Proj.Qualis[qIdx]
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
				for i, want := 1, 5; numpages >= want && len(shownums) < want && i < numpages; i++ {
					if len(shownums) < want && (pageNr+i) < numpages {
						shownums[pageNr+i] = true
					}
					if len(shownums) < want && (pageNr-i) > 1 {
						shownums[pageNr-i] = true
					}
				}
			}
			for i := range chapter.sheets {
				if 0 == (i % chapter.SheetsPerPage) {
					pgnr++
					if pgnr == pageNr {
						s += "<li><b>" + itoa(pgnr) + "</b></li>"
					} else if shownums[pgnr] {
						name := me.namePage(series, chapter, quali.SizeHint, pgnr, viewMode, me.lang)
						s += "<li><a href='./" + name + ".html#" + App.Proj.Gen.APaging + "'>" + itoa(pgnr) + "</a></li>"
					}
				}
				if pgnr == pageNr && istoplist {
					sheets = append(sheets, chapter.sheets[i])
				}
			}
		}
		if pageNr == numpages && len(series.Chapters) > 1 {
			chidx := 0
			for i, chap := range series.Chapters {
				if chap == chapter {
					chidx = i
					break
				}
			}
			nextchap := series.Chapters[0]
			if chidx < len(series.Chapters)-1 {
				nextchap = series.Chapters[chidx+1]
			}
			name := me.namePage(series, nextchap, quali.SizeHint, 1, viewMode, me.lang)
			s += "<li><a href='./" + name + ".html#" + App.Proj.Gen.APaging + "'>" + locStr(nextchap.Title, me.lang) + "</a></li>"
		}
		if s != "" {
			var pg int
			if pg = pageNr - 1; pg < 1 {
				pg = 1
			}
			pvis, prev := "hidden", me.namePage(series, chapter, quali.SizeHint, pg, viewMode, me.lang)
			if pg = pageNr + 1; pg > numpages {
				pg = numpages
			}
			nvis, next := "hidden", me.namePage(series, chapter, quali.SizeHint, pg, viewMode, me.lang)
			if pageNr > 1 {
				pvis = "visible"
			}
			if pageNr < numpages {
				nvis = "visible"
			}
			ulid := App.Proj.Gen.APaging
			if !istoplist {
				ulid += "b"
			}
			s = "<ul id='" + ulid + "'><li><a style='visibility: " + pvis + "' href='./" + strings.ToLower(prev) + ".html#" + App.Proj.Gen.APaging + "'>&larr;</a></li>" + s + "<li><a style='visibility: " + nvis + "' href='./" + strings.ToLower(next) + ".html#" + App.Proj.Gen.APaging + "'>&rarr;</a></li></ul>"
		}
		return s
	}
	me.page.PagesList, me.page.PageContent = pageslist(), "<div class='"+App.Proj.Gen.ClsViewerPage+"'>"

	me.page.ViewerList = ""
	for _, viewmode := range viewModes {
		if me.page.ViewerList += "<div title='" + me.textStr("ViewMode_"+viewmode) + "' class='v" + viewmode; viewmode == viewMode {
			me.page.ViewerList += " vc"
		}
		me.page.ViewerList += "'>"
		if viewmode == viewMode {
			me.page.ViewerList += "<b>&nbsp;</b>"
		} else {
			me.page.ViewerList += "<a href='./" + me.namePage(series, chapter, App.Proj.Qualis[qIdx].SizeHint, pageNr, viewmode, me.lang) + ".html#paging'>&nbsp;</a>"
		}
		me.page.ViewerList += "</div>"
	}

	var iter func(*SheetVer, *ImgPanel) string
	pidx, allpanels := 0, map[string]int{}
	iter = func(sheetVer *SheetVer, panel *ImgPanel) (s string) {
		assert(len(panel.SubCols) == 0 || len(panel.SubRows) == 0)
		px1cm := float64(sheetVer.data.PanelsTree.Rect.Max.Y-sheetVer.data.PanelsTree.Rect.Min.Y) / 21.0
		if len(panel.SubRows) > 0 {
			for i := range panel.SubRows {
				sr := &panel.SubRows[i]
				s += "<div class='" + App.Proj.Gen.ClsPanelRow + "'>" + iter(sheetVer, sr) + "</div>"
			}
		} else if len(panel.SubCols) > 0 {
			for i := range panel.SubCols {
				sc := &panel.SubCols[i]
				pw, sw := sc.Rect.Max.X-sc.Rect.Min.X, panel.Rect.Max.X-panel.Rect.Min.X
				pp := 100.0 / (float64(sw) / float64(pw))
				s += "<div class='" + App.Proj.Gen.ClsPanelCol + "' style='width: " + strconv.FormatFloat(pp, 'f', 8, 64) + "%'>" + iter(sheetVer, sc) + "</div>"
			}
		} else {
			allpanels[App.Proj.data.ContentHashes[sheetVer.fileName]] = pidx
			hqsrc, name := "", strings.ToLower(App.Proj.data.ContentHashes[sheetVer.fileName]+itoa(App.Proj.Qualis[0].SizeHint)+itoa(pidx))
			for i := qIdx; i >= 0; i-- {
				hqsrc = strings.ToLower(App.Proj.data.ContentHashes[sheetVer.fileName] + itoa(App.Proj.Qualis[i].SizeHint) + itoa(pidx))
				if fileinfo, err := os.Stat(".build/img/" + hqsrc + ".png"); err == nil && (!fileinfo.IsDir()) && fileinfo.Size() > 0 {
					break
				}
			}
			if hqsrc == name {
				hqsrc = ""
			}

			s += "<div class='" + App.Proj.Gen.ClsPanel + "'>"
			s += me.genSvgForPanel(sheetVer, pidx, panel, px1cm)
			s += "<img id='" + name + "' src='./img/" + name + ".png' class='" + App.Proj.Gen.ClsImgHq + "' " + App.Proj.Gen.ClsImgHq + "='" + hqsrc + "'/>"

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
		assert(len(sheet.versions) == 1)
		sheetver := sheet.versions[0]
		sheetver.ensurePrep(false, false)
		pidx = 0
		if viewMode != "r" {
			me.page.PageContent += "<div class='" + App.Proj.Gen.ClsSheet + "'>"
		}
		me.page.PageContent += iter(sheetver, sheetver.data.PanelsTree)
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

func (me *siteGen) genSvgForPanel(sheetVer *SheetVer, panelIdx int, panel *ImgPanel, px1cm float64) string {
	panelareas := sheetVer.panelAreas(panelIdx)
	if len(panelareas) == 0 {
		return ""
	}

	pw, ph := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y
	s := "<svg onload='this.style.visibility=\"hidden\";' viewbox='0 0 " + itoa(pw) + " " + itoa(ph) + "'>"
	for _, pta := range panelareas {
		rx, ry, rw, rh := pta.Rect.Min.X-panel.Rect.Min.X, pta.Rect.Min.Y-panel.Rect.Min.Y, pta.Rect.Max.X-pta.Rect.Min.X, pta.Rect.Max.Y-pta.Rect.Min.Y
		borderandfill := pta.PointTo != nil
		if borderandfill {
			rpx, rpy := pta.PointTo.X-panel.Rect.Min.X, pta.PointTo.Y-panel.Rect.Min.Y
			mmh, cmh := int(px1cm*App.Proj.Gen.PanelSvgText.BoxPolyStrokeWidthCm), int(px1cm/2.0)
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
		linex := 0
		if borderandfill {
			linex = int(px1cm / 11.0)
		}
		s += imgSvgText(&pta, me.lang, px1cm, false, linex)
		s += "</svg>"
	}

	s += "</svg>"
	return s
}

func (me *siteGen) genPageExecAndWrite(name string) (numFilesWritten int) {
	me.page.LangsList = ""
	for _, lang := range App.Proj.Langs {
		me.page.LangsList += "<div>"
		if lang == me.lang {
			me.page.LangsList += "<b><img title='" + lang + "' alt='" + lang + "' src='./l" + lang + ".svg'/></b>"
		} else {
			href := name[:len(name)-len(me.lang)] + lang
			if name == "index" && me.lang == App.Proj.Langs[0] {
				href = name + "." + lang
			} else if lang == App.Proj.Langs[0] && strings.HasPrefix(name, "index.") {
				href = "index"
			}
			me.page.LangsList += "<a href='./" + href + ".html'><img alt='" + lang + "' title='" + lang + "' src='./l" + lang + ".svg'/></a>"
		}
		me.page.LangsList += "</div>"
	}
	if me.page.PageTitleTxt == "" {
		me.page.PageTitleTxt = me.page.PageTitle
	}

	buf := bytes.NewBuffer(nil)
	if err := me.tmpl.ExecuteTemplate(buf, "_tmpl.html", &me.page); err != nil {
		panic(err)
	}
	writeFile(".build/"+strings.ToLower(name)+".html", buf.Bytes())
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

var svgRepl = strings.NewReplacer(
	" ", "&nbsp;",
	"<b>", "<tspan class='b'>",
	"<u>", "<tspan class='u'>",
	"<i>", "<tspan class='i'>",
	"</b>", "</tspan>",
	"</u>", "</tspan>",
	"</i>", "</tspan>",
)

func (me *siteGen) genAtomXml() (numFilesWritten int) {
	af := App.Proj.AtomFile
	s := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom" xml:lang="` + me.lang + `">`
	var latestdate string
	var xmls []string
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			if len(chapter.History) > 0 {
				for _, entry := range chapter.History {
					if entry.Date > latestdate {
						latestdate = entry.Date
					}
					xml := `<entry><updated>` + entry.Date + `T00:00:00Z</updated>`
					xml += `<title>Update: ` + hEsc(locStr(series.Title, me.lang)) + ` - ` + hEsc(locStr(chapter.Title, me.lang)) + `</title>`
					xml += `<content type="html">` + hEsc(locStr(entry.Notes, me.lang)) + `<hr/>&quot;` + hEsc(locStr(series.Desc, me.lang)) + `&quot;</content>`
					xml += `<link href="` + strings.TrimRight(af.LinkHref, "/") + "/" + me.namePage(series, chapter, App.Proj.Qualis[0].SizeHint, entry.PageNr, "s", me.lang) + ".html" + `"/>`
					xml += `<author><name>` + af.Title + `</name></author>`
					xmls = append(xmls, xml+`</entry>`)
				}
			}
		}
	}

	if latestdate != "" {
		s += `<updated>` + latestdate + `T00:00:00Z</updated><title>` + af.Title + `</title><link href="` + af.LinkHref + `"/><id>` + af.LinkHref + "</id>"
		sort.Strings(xmls)
		for i := len(xmls) - 1; i >= 0; i-- {
			s += xmls[i]
		}
		writeFile(".build/"+af.Name+"."+me.lang+".atom", []byte(s+"</feed>"))
		numFilesWritten++
	}
	return
}

func (me *siteGen) genThumbsPngs() (numPngs int) {
	var work sync.WaitGroup
	work.Add(len(App.Proj.Series))
	for _, series := range App.Proj.Series {
		go func(series *Series) {
			var filenames []string
			for _, chapter := range series.Chapters {
				for _, sheet := range chapter.sheets {
					sv := sheet.versions[len(sheet.versions)-1]
					filenames = append(filenames, sv.data.bwFilePath)
				}
			}
			if App.Proj.NumSheetsInHomeBgs > 0 && len(filenames) > App.Proj.NumSheetsInHomeBgs {
				filenames = filenames[:App.Proj.NumSheetsInHomeBgs]
			}

			data := imgStitchHorizontally(filenames, 640, 44, color.NRGBA{0, 0, 0, 0})
			writeFile("./.build/img/s"+itoa(App.Proj.NumSheetsInHomeBgs)+strings.ToLower(series.Name)+".png", data)
			work.Done()
		}(series)
	}
	work.Wait()
	return len(App.Proj.Series)
}

func (me *siteGen) namePage(series *Series, chapter *Chapter, quali int, pageNr int, viewMode string, langId string) string {
	if pageNr < 1 {
		pageNr = 1
	}
	return strings.ToLower(series.Name + "-" + chapter.Name + "-" + itoa(quali) + viewMode + itoa(pageNr) + "." + langId)
}
