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
}

func siteGen(args map[string]bool) {
	tstart := time.Now()
	printLn("SiteGen started. When done, result will open in new window.")
	defer func() {
		if err := recover(); err != nil {
			printLn("SiteGen Error: ", err)
		}
	}()

	if !args["nopngs"] {
		rmDir(".build")
		mkDir(".build")
		mkDir(".build/img/")
	}

	printLn("SiteGen: copying non-HTML files to .build...")
	modifycssfiles := App.Proj.Gen.PanelSvgText.AppendToFiles
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
				}
			}
		}
	}

	if !args["nopngs"] {
		tstartpngs := time.Now()
		printLn("SiteGen: generating PNGs...")
		numpngs, numsheets, numpanels := siteGenPngs()
		numpngs += siteGenThumbs()
		printLn("SiteGen took " + time.Now().Sub(tstartpngs).String() + " for generating all " + itoa(numpngs) + " PNGs for " + itoa(numpanels) + " panels from " + itoa(numsheets) + " sheets")
	}

	printLn("SiteGen: generating HTML files...")
	tmpl, err := template.New("foo").ParseFiles("sitetmpl/_tmpl.html")
	if err != nil {
		panic(err)
	}
	for _, lang := range App.Proj.Langs {
		siteGenPages(tmpl, nil, nil, lang, 0)
		for _, series := range App.Proj.Series {
			for _, chapter := range series.Chapters {
				if chapter.SheetsPerPage > 0 {
					for i := 1; i <= (len(chapter.sheets) / chapter.SheetsPerPage); i++ {
						siteGenPages(tmpl, series, chapter, lang, i)
					}
				} else {
					siteGenPages(tmpl, series, chapter, lang, 0)
				}
			}
		}
	}
	if App.Proj.AtomFile.Name != "" {
		for _, lang := range App.Proj.Langs {
			siteGenAtom(lang)
		}
	}

	printLn("SiteGen: DONE after " + time.Now().Sub(tstart).String())
	browserCmd[len(browserCmd)-1] = "--app=file://" + os.Getenv("PWD") + "/.build/index.html"
	cmd := exec.Command(browserCmd[0], browserCmd[1:]...)
	if err := cmd.Run(); err != nil {
		printLn(err)
	}
}

func siteGenPngs() (numPngs int, numSheets int, numPanels int) {
	var numpngs atomic.Value
	numpngs.Store(0)
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			for _, sheet := range chapter.sheets {
				for _, sheetver := range sheet.versions {
					numSheets++
					sheetver.ensurePrep(false, false)
					srcimgfile, err := os.Open(sheetver.meta.bwFilePath)
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
					contenthash := App.Proj.meta.ContentHashes[sheetver.fileName]
					sheetver.meta.PanelsTree.iter(func(panel *ImgPanel) {
						work.Add(1)
						numPanels++
						go func(pidx int) {
							for _, quali := range App.Proj.Qualis {
								name := strings.ToLower(contenthash + itoa(quali.SizeHint) + itoa(pidx))
								pw, ph, sw := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y, sheetver.meta.PanelsTree.Rect.Max.X-sheetver.meta.PanelsTree.Rect.Min.X
								width := float64(quali.SizeHint) / (float64(sw) / float64(pw))
								height := width / (float64(pw) / float64(ph))
								w, h := int(width), int(height)
								var wassamesize bool
								writeFile(".build/img/"+name+".png", imgSubRectPng(imgsrc.(*image.Gray), panel.Rect, &w, &h, quali.SizeHint/640, 0, sheetver.colorLayers, &wassamesize))
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

func siteGenPages(tmpl *template.Template, series *Series, chapter *Chapter, langId string, pageNr int) {
	assert((series == nil) == (chapter == nil))

	name, page := "index", PageGen{
		SiteTitle:  hEsc(App.Proj.Title),
		SiteDesc:   hEsc(siteGenLocStr(App.Proj.Desc, langId)),
		PageLang:   langId,
		FooterHtml: siteGenTextStr("FooterHtml", langId),
		HrefHome:   "./index.html",
	}
	if langId != App.Proj.Langs[0] {
		name += "-" + langId
		page.HrefHome = "./index-" + langId + ".html"
	}

	if series == nil && chapter == nil {
		page.PageTitle = hEsc(siteGenTextStr("HomeTitle", langId))
		page.PageDesc = hEsc(siteGenTextStr("HomeDesc", langId))
		sitePrepHomePage(&page, langId)
		siteGenPageExecAndWrite(tmpl, name, langId, &page)
	} else {
		page.PageCssClasses = "chapter"
		page.HrefHome += "#" + strings.ToLower(series.Name)
		page.PageTitle = "<span>" + hEsc(siteGenLocStr(series.Title, langId)) + ":</span> " + hEsc(siteGenLocStr(chapter.Title, langId))
		page.PageTitleTxt = hEsc(siteGenLocStr(series.Title, langId)) + ": " + hEsc(siteGenLocStr(chapter.Title, langId))
		var authorinfo string
		if series.Author != "" {
			authorinfo = strings.Replace(siteGenTextStr("TmplAuthorInfoHtml", langId), "%AUTHOR%", series.Author, 1)
		}
		page.PageDesc = hEsc(siteGenLocStr(series.Desc, langId)) + authorinfo
		for qidx, quali := range App.Proj.Qualis {
			name = series.Name + "-" + chapter.Name + "-" + itoa(quali.SizeHint)
			if pageNr != 0 {
				name += "-p" + itoa(pageNr)
			} else {
				name += "-p1"
			}
			name += "-" + langId

			allpanels := sitePrepSheetPage(&page, langId, qidx, series, chapter, pageNr)
			page.QualList = ""
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

				href := strings.Replace(name, "-"+itoa(quali.SizeHint)+"-", "-"+itoa(q.SizeHint)+"-", 1)
				page.QualList += "<option value='" + strings.ToLower(href) + "'"
				if q.Name == quali.Name {
					page.QualList += " selected='selected'"
				}
				imgsizeinfo := itoa(int(totalimgsize/1024)) + "KB"
				if mb := totalimgsize / 1048576; mb > 0 {
					imgsizeinfo = strconv.FormatFloat(float64(totalimgsize)/1048576.0, 1, 'f', 64) + "MB"
				}
				page.QualList += ">" + q.Name + " (" + imgsizeinfo + ")" + "</option>"
			}
			page.QualList = "<select title='" + hEsc(siteGenTextStr("QualityHint", langId)) + "' name='" + App.Proj.Gen.IdQualiList + "' id='" + App.Proj.Gen.IdQualiList + "'>" + page.QualList + "</select>"

			siteGenPageExecAndWrite(tmpl, name, langId, &page)
		}
	}
}

func siteGenPageExecAndWrite(tmpl *template.Template, name string, langId string, page *PageGen) {
	page.LangsList = ""
	for _, lang := range App.Proj.Langs {
		page.LangsList += "<div>"
		if lang == langId {
			page.LangsList += "<b><img title='" + lang + "' alt='" + lang + "' src='./l" + "ang" + "-" + lang + ".s" + "vg'/></b>"
		} else {
			href := name[:len(name)-len(langId)] + lang
			if name == "index" && langId == App.Proj.Langs[0] {
				href = name + "-" + lang
			} else if lang == App.Proj.Langs[0] && strings.HasPrefix(name, "index-") {
				href = "index"
			}
			page.LangsList += "<a href='./" + strings.ToLower(href) + ".html'><img alt='" + lang + "' title='" + lang + "' src='./la" + "ng" + "-" + lang + ".sv" + "g'/></a>"
		}
		page.LangsList += "</div>"
	}
	if page.PageTitleTxt == "" {
		page.PageTitleTxt = page.PageTitle
	}

	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, "_tmpl.html", page); err != nil {
		panic(err)
	}
	writeFile(".build/"+strings.ToLower(name)+".html", buf.Bytes())
}

func siteGenLocStr(m map[string]string, langId string) (s string) {
	if s = m[langId]; s == "" {
		s = m[App.Proj.Langs[0]]
	}
	return s
}

func siteGenTextStr(key string, langId string) (s string) {
	if s = App.Proj.PageContentTexts[langId][key]; s == "" {
		if s = App.Proj.PageContentTexts[App.Proj.Langs[0]][key]; s == "" {
			s = key
		}
	}
	return s
}

func sitePrepHomePage(page *PageGen, langId string) {
	page.PageContent = ""
	for _, series := range App.Proj.Series {
		var authorinfo string
		if series.Author != "" {
			authorinfo = strings.Replace(siteGenTextStr("TmplAuthorInfoHtml", langId), "%AUTHOR%", series.Author, 1)
		}
		page.PageContent += "<span class='" + App.Proj.Gen.ClsSeries + "' style='background-image: url(\"./img/s" + itoa(App.Proj.NumSheetsInHomeBgs) + strings.ToLower(series.Name) + ".png\");'><span><h5 id='" + strings.ToLower(series.Name) + "' class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(siteGenLocStr(series.Title, langId)) + "</h5><div class='" + App.Proj.Gen.ClsSeries + "'>" + hEsc(siteGenLocStr(series.Desc, langId)) + authorinfo + "</div>"
		page.PageContent += "<ul class='" + App.Proj.Gen.ClsSeries + "'>"
		for _, chapter := range series.Chapters {
			page.PageContent += "<li class='" + App.Proj.Gen.ClsChapter + "'><a href='./" + strings.ToLower(series.Name+"-"+chapter.Name+"-"+itoa(App.Proj.Qualis[0].SizeHint)+"-p1-"+langId) + ".html'>" + hEsc(siteGenLocStr(chapter.Title, langId)) + "</a></li>"
		}
		page.PageContent += "</ul></span><div></div></span>"
	}
}

func sitePrepSheetPage(page *PageGen, langId string, qIdx int, series *Series, chapter *Chapter, pageNr int) map[string]int {
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
						href := series.Name + "-" + chapter.Name + "-" + itoa(quali.SizeHint) + "-p" + itoa(pgnr) + "-" + langId
						s += "<li><a href='./" + strings.ToLower(href) + ".html#" + App.Proj.Gen.APaging + "'>" + itoa(pgnr) + "</a></li>"
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
			href := series.Name + "-" + nextchap.Name + "-" + itoa(quali.SizeHint) + "-p1-" + langId
			s += "<li><a href='./" + strings.ToLower(href) + ".html#" + App.Proj.Gen.APaging + "'>" + siteGenLocStr(nextchap.Title, langId) + "</a></li>"
		}
		if s != "" {
			var pg int
			if pg = pageNr - 1; pg < 1 {
				pg = 1
			}
			pvis, prev := "hidden", series.Name+"-"+chapter.Name+"-"+itoa(quali.SizeHint)+"-p"+itoa(pg)+"-"+langId
			if pg = pageNr + 1; pg > numpages {
				pg = numpages
			}
			nvis, next := "hidden", series.Name+"-"+chapter.Name+"-"+itoa(quali.SizeHint)+"-p"+itoa(pg)+"-"+langId
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
	page.PagesList, page.PageContent = pageslist(), "<div class='"+App.Proj.Gen.ClsSheetsPage+"'>"

	var pidx int
	var iter func(*SheetVer, *ImgPanel) string
	allpanels := map[string]int{}
	iter = func(sheetVer *SheetVer, panel *ImgPanel) (s string) {
		assert(len(panel.SubCols) == 0 || len(panel.SubRows) == 0)
		px1cm := float64(sheetVer.meta.PanelsTree.Rect.Max.Y-sheetVer.meta.PanelsTree.Rect.Min.Y) / 21.0
		if len(panel.SubRows) > 0 {
			s += "<div class='" + App.Proj.Gen.ClsPanelRows + "'>"
			for i := range panel.SubRows {
				s += "<div class='" + App.Proj.Gen.ClsPanelRow + "'>" + iter(sheetVer, &panel.SubRows[i]) + "</div>"
			}
			s += "</div>"
		} else if len(panel.SubCols) > 0 {
			s += "<div class='" + App.Proj.Gen.ClsPanelCols + "'>"
			for i := range panel.SubCols {
				sc := &panel.SubCols[i]
				pw := sc.Rect.Max.X - sc.Rect.Min.X
				sw := panel.Rect.Max.X - panel.Rect.Min.X
				pp := 100.0 / (float64(sw) / float64(pw))
				s += "<div class='" + App.Proj.Gen.ClsPanelCol + "' style='width: " + strconv.FormatFloat(pp, 'f', 8, 64) + "%'>" + iter(sheetVer, sc) + "</div>"
			}
			s += "</div>"
		} else {
			allpanels[App.Proj.meta.ContentHashes[sheetVer.fileName]] = pidx
			var name string
			for i := qIdx; i >= 0; i-- {
				name = strings.ToLower(App.Proj.meta.ContentHashes[sheetVer.fileName] + itoa(App.Proj.Qualis[i].SizeHint) + itoa(pidx))
				if fileinfo, err := os.Stat(".build/img/" + name + ".png"); err == nil && (!fileinfo.IsDir()) && fileinfo.Size() > 0 {
					break
				}
			}

			s += "<div class='" + App.Proj.Gen.ClsPanel + "'>"
			s += "<img id='" + name + "' src='./img/" + name + ".png'/>"
			for _, pta := range panel.Areas {
				aw, ah := pta.Rect.Max.X-pta.Rect.Min.X, pta.Rect.Max.Y-pta.Rect.Min.Y
				pw, ph := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y
				wp, hp := 100.0/(float64(pw)/float64(aw)), 100.0/(float64(ph)/float64(ah))
				ax, ay := pta.Rect.Min.X-panel.Rect.Min.X, pta.Rect.Min.Y-panel.Rect.Min.Y
				xp, yp := 100.0/(float64(pw)/float64(ax)), 100.0/(float64(ph)/float64(ay))
				sx, sy, sw, sh := strconv.FormatFloat(xp, 'f', 8, 64), strconv.FormatFloat(yp, 'f', 8, 64), strconv.FormatFloat(wp, 'f', 8, 64), strconv.FormatFloat(hp, 'f', 8, 64)
				s += "<div class='" + App.Proj.Gen.ClsPanelArea + "' style='top: " + sy + "%; left: " + sx + "%; width: " + sw + "%; height: " + sh + "%; max-width: " + sw + "%; max-height: " + sh + "%;'>"
				s += imgSvgText(&pta, langId, px1cm)
				s += "</div>"
			}
			s += "</div>"
			pidx++
		}
		return
	}
	page.PageContent += "<div class='" + App.Proj.Gen.ClsSheets + "'>"
	for _, sheet := range sheets {
		assert(len(sheet.versions) == 1)
		sheetver := sheet.versions[0]
		sheetver.ensurePrep(false, false)
		pidx = 0
		page.PageContent += "<div class='" + App.Proj.Gen.ClsSheet + "'>"
		page.PageContent += iter(sheetver, sheetver.meta.PanelsTree)
		page.PageContent += "</div>"
	}
	page.PageContent += "</div>"
	if len(sheets) > 1 {
		page.PageContent += pageslist()
	}
	page.PageContent += "</div>"

	return allpanels
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

func siteGenAtom(lang string) {
	af := App.Proj.AtomFile
	s := `<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom" xml:lang="` + lang + `">`
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
					xml += `<title>Update: ` + hEsc(siteGenLocStr(series.Title, lang)) + ` - ` + hEsc(siteGenLocStr(chapter.Title, lang)) + `</title>`
					xml += `<content type="html">` + hEsc(siteGenLocStr(entry.Notes, lang)) + `<hr/>&quot;` + hEsc(siteGenLocStr(series.Desc, lang)) + `&quot;</content>`
					xml += `<link href="` + strings.TrimRight(af.LinkHref, "/") + "/" + strings.ToLower(series.Name+"-"+chapter.Name+"-12"+"80-p"+itoa(entry.PageNr)+"-"+lang) + ".html" + `"/>`
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
		writeFile(filepath.Join(".build", af.Name+"."+lang+".xml"), []byte(s+"</feed>"))
	}
}

func siteGenThumbs() (numPngs int) {
	var work sync.WaitGroup
	work.Add(len(App.Proj.Series))
	for _, series := range App.Proj.Series {
		go func(series *Series) {
			var filenames []string
			for _, chapter := range series.Chapters {
				for _, sheet := range chapter.sheets {
					sv := sheet.versions[len(sheet.versions)-1]
					filenames = append(filenames, sv.meta.bwFilePath)
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
