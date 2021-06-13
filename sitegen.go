package main

import (
	"bytes"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

type PageGen struct {
	SiteTitle   string
	SiteDesc    string
	PageTitle   string
	PageDesc    string
	LangsList   string
	QualList    string
	PagesList   string
	PageContent string
	FooterHtml  string
	HrefHome    string
}

func siteGen() {
	printLn("SiteGen started. When done, result will open in new window.")
	defer func() {
		if err := recover(); err != nil {
			printLn("SiteGen Error: ", err)
		}
	}()

	if err := os.RemoveAll(".build"); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	mkDir(".build")
	mkDir(".build/img/")

	printLn("SiteGen: copying non-HTML files to .build...")
	if fileinfos, err := os.ReadDir("_sitetmpl"); err != nil {
		panic(err)
	} else {
		for _, fileinfo := range fileinfos {
			if !(fileinfo.IsDir() || strings.Contains(strings.ToLower(filepath.Ext(fileinfo.Name())), "htm")) {
				if data, err := os.ReadFile("_sitetmpl/" + fileinfo.Name()); err != nil {
					panic(err)
				} else if err := os.WriteFile(".build/"+fileinfo.Name(), data, os.ModePerm); err != nil {
					panic(err)
				}
			}
		}
	}

	numsvgs, tstartsvgs := 0, time.Now()
	printLn("SiteGen: generating SVGs...")
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			for _, sheet := range chapter.sheets {
				for _, sheetver := range sheet.versions {
					sheetver.ensure(true)
					srcimgfile, err := os.Open(sheetver.meta.bwFilePath)
					if err != nil {
						panic(err)
					}
					imgsrc, _, err := image.Decode(srcimgfile)
					if err != nil {
						panic(err)
					}
					_ = srcimgfile.Close()

					var work sync.WaitGroup
					for lidx, lang := range App.Proj.Langs {
						for _, quali := range App.Proj.Qualis {
							work.Add(1)
							go func(lidx int, langName string, quali int) {
								var pidx int
								sheetver.meta.PanelsTree.iter(func(panel *ImgPanel) {
									if lidx == 0 || panel.HasAny(langName) {
										tstart := time.Now()
										name := strings.ToLower(App.Proj.meta.ContentHashes[sheetver.fileName]+langName+itoa(quali)+itoa(pidx)) + ".svg"
										pw, ph, sw := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y, sheetver.meta.PanelsTree.Rect.Max.X-sheetver.meta.PanelsTree.Rect.Min.X
										width := float64(quali) / (float64(sw) / float64(pw))
										height := width / (float64(pw) / float64(ph))
										writeFile(".build/img/"+name, []byte(imgSubRectSvg(imgsrc.(*image.Gray), panel.Rect, int(width), int(height))))
										numsvgs++
										printLn("\t", name+" ("+time.Now().Sub(tstart).String()+")")
									}
									pidx++
								})
								work.Done()
							}(lidx, lang.Name, quali.SizeHint)
						}
					}
					work.Wait()
				}
			}
		}
	}
	printLn("SiteGen took " + time.Now().Sub(tstartsvgs).String() + " for generating all " + itoa(numsvgs) + " SVGs")

	printLn("SiteGen: generating HTML files...")
	tmpl, err := template.New("foo").ParseFiles("_sitetmpl/_tmpl.html")
	if err != nil {
		panic(err)
	}
	for _, lang := range App.Proj.Langs {
		siteGenPages(tmpl, nil, nil, lang.Name, 0)
		for _, series := range App.Proj.Series {
			for _, chapter := range series.Chapters {
				if chapter.SheetsPerPage > 0 {
					for i := 1; i <= (len(chapter.sheets) / chapter.SheetsPerPage); i++ {
						siteGenPages(tmpl, series, chapter, lang.Name, i)
					}
				} else {
					siteGenPages(tmpl, series, chapter, lang.Name, 0)
				}
			}
		}
	}

	printLn("SiteGen: DONE!")
	browserCmd[len(browserCmd)-1] = "--app=file://" + os.Getenv("PWD") + "/.build/index.html"
	cmd := exec.Command(browserCmd[0], browserCmd[1:]...)
	if err := cmd.Run(); err != nil {
		printLn(err)
	}
}

func siteGenPages(tmpl *template.Template, series *Series, chapter *Chapter, langId string, pageNr int) {
	assert((series == nil) == (chapter == nil))

	name, page := "index", PageGen{
		SiteTitle:  hEsc(App.Proj.Title),
		SiteDesc:   hEsc(siteGenLocStr(App.Proj.Desc, langId)),
		FooterHtml: siteGenTextStr("FooterHtml", langId),
		HrefHome:   "./index.html",
	}
	if langId != App.Proj.Langs[0].Name {
		name += "-" + langId
		page.HrefHome = "./index-" + langId + ".html"
	}

	if series == nil && chapter == nil {
		page.PageTitle = hEsc(siteGenTextStr("HomeTitle", langId))
		page.PageDesc = hEsc(siteGenTextStr("HomeDesc", langId))
		sitePrepHomePage(&page, langId)
		siteGenPageExecAndWrite(tmpl, name, langId, &page)
	} else {
		var authorinfo string
		if series.Author != "" {
			authorinfo = " (Story: © " + series.Author + ")"
		}
		page.PageTitle = hEsc(siteGenLocStr(series.Title, langId)) + ": " + hEsc(siteGenLocStr(chapter.Title, langId))
		page.PageDesc = hEsc(siteGenLocStr(series.Desc, langId) + authorinfo)
		for qidx, quali := range App.Proj.Qualis {
			name = series.Name + "-" + chapter.Name + "-" + quali.Name
			if pageNr != 0 {
				name += "-p" + itoa(pageNr)
			} else {
				name += "-p1"
			}
			name += "-" + langId

			page.QualList = ""
			for _, q := range App.Proj.Qualis {
				href := strings.Replace(name, "-"+quali.Name+"-", "-"+q.Name+"-", 1)
				page.QualList += "<option value='" + strings.ToLower(href) + "'"
				if q.Name == quali.Name {
					page.QualList += " selected='selected'"
				}
				page.QualList += ">" + q.Name + "</option>"
			}
			page.QualList = "<select name='" + App.Proj.Html.IdQualiList + "' id='" + App.Proj.Html.IdQualiList + "'>" + page.QualList + "</select>"

			sitePrepSheetPage(&page, langId, qidx, series, chapter, pageNr)
			siteGenPageExecAndWrite(tmpl, name, langId, &page)
		}
	}
}

func siteGenPageExecAndWrite(tmpl *template.Template, name string, langId string, page *PageGen) {
	// println("\t", strings.ToLower(name)+".html...")
	page.LangsList = ""
	for _, lang := range App.Proj.Langs {
		page.LangsList += "<li>"
		if lang.Name == langId {
			page.LangsList += "<b>" + hEsc(lang.Title) + "</b>"
		} else {
			href := name[:len(name)-len(langId)] + lang.Name
			if name == "index" && langId == App.Proj.Langs[0].Name {
				href = name + "-" + lang.Name
			} else if lang.Name == App.Proj.Langs[0].Name && strings.HasPrefix(name, "index-") {
				href = "index"
			}
			page.LangsList += "<a href='./" + strings.ToLower(href) + ".html'>" + hEsc(lang.Title) + "</a>"
		}
		page.LangsList += "</li>"
	}
	page.LangsList = "<ul>" + page.LangsList + "</ul>"

	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, "_tmpl.html", page); err != nil {
		panic(err)
	}
	writeFile(".build/"+strings.ToLower(name)+".html", buf.Bytes())
}

func siteGenLocStr(m map[string]string, langId string) (s string) {
	if s = m[langId]; s == "" {
		s = m[App.Proj.Langs[0].Name]
	}
	return s
}

func siteGenTextStr(key string, langId string) (s string) {
	if s = App.Proj.PageContentTexts[langId][key]; s == "" {
		if s = App.Proj.PageContentTexts[App.Proj.Langs[0].Name][key]; s == "" {
			s = key
		}
	}
	return s
}

func sitePrepHomePage(page *PageGen, langId string) {
	page.PageContent = ""
	for _, series := range App.Proj.Series {
		var authordiv string
		if series.Author != "" {
			authordiv = "<div class='" + App.Proj.Html.ClsSeries + "'>(Story: &copy; " + series.Author + ")</div>"
		}
		page.PageContent += "<h5 class='" + App.Proj.Html.ClsSeries + "'>" + hEsc(siteGenLocStr(series.Title, langId)) + "</h5><div class='" + App.Proj.Html.ClsSeries + "'>" + hEsc(siteGenLocStr(series.Desc, langId)) + "</div>" + authordiv
		page.PageContent += "<ul class='" + App.Proj.Html.ClsSeries + "'>"
		for _, chapter := range series.Chapters {
			page.PageContent += "<li class='" + App.Proj.Html.ClsChapter + "'><a href='./" + series.Name + "-" + chapter.Name + "-fhd-p1-" + langId + ".html'>" + hEsc(siteGenLocStr(chapter.Title, langId)) + "</a></li>"
		}
		page.PageContent += "</ul>"
	}
}

func sitePrepSheetPage(page *PageGen, langId string, qIdx int, series *Series, chapter *Chapter, pageNr int) {
	quali := App.Proj.Qualis[qIdx]
	page.PagesList, page.PageContent = "", ""
	var sheets []*Sheet
	numpages := 1
	switch chapter.SheetsPerPage {
	case 0:
		sheets = chapter.sheets
	default:
		numpages = len(chapter.sheets) / chapter.SheetsPerPage
		var pgnr int
		shownums := map[int]bool{1: true, numpages: true, pageNr: true}
		for i, want := 1, 5; numpages >= want && len(shownums) < want && i < numpages; i++ {
			if len(shownums) < want && (pageNr+i) < numpages {
				shownums[pageNr+i] = true
			}
			if len(shownums) < want && (pageNr-i) > 1 {
				shownums[pageNr-i] = true
			}
		}
		for i := range chapter.sheets {
			if 0 == (i % chapter.SheetsPerPage) {
				pgnr++
				if pgnr == pageNr {
					page.PagesList += "<li><b>" + itoa(pgnr) + "</b></li>"
				} else if shownums[pgnr] {
					href := series.Name + "-" + chapter.Name + "-" + quali.Name + "-p" + itoa(pgnr) + "-" + langId
					page.PagesList += "<li><a href='./" + strings.ToLower(href) + ".html'>" + itoa(pgnr) + "</a></li>"
				}
			}
			if pgnr == pageNr {
				sheets = append(sheets, chapter.sheets[i])
			}
		}
	}
	if page.PagesList != "" {
		var pg int
		if pg = pageNr - 1; pg < 1 {
			pg = 1
		}
		pvis, prev := "hidden", series.Name+"-"+chapter.Name+"-"+quali.Name+"-p"+itoa(pg)+"-"+langId
		if pg = pageNr + 1; pg > numpages {
			pg = numpages
		}
		nvis, next := "hidden", series.Name+"-"+chapter.Name+"-"+quali.Name+"-p"+itoa(pg)+"-"+langId
		if pageNr > 1 {
			pvis = "visible"
		}
		if pageNr < numpages {
			nvis = "visible"
		}
		page.PagesList = "<ul><li><a style='visibility: " + pvis + "' href='./" + strings.ToLower(prev) + ".html'>&larr;</a></li>" + page.PagesList + "<li><a style='visibility: " + nvis + "' href='./" + strings.ToLower(next) + ".html'>&rarr;</a></li></ul>"
	}

	var pidx int
	var iter func(*SheetVer, *ImgPanel) string
	iter = func(sheetVer *SheetVer, panel *ImgPanel) (s string) {
		assert(len(panel.SubCols) == 0 || len(panel.SubRows) == 0)
		if len(panel.SubRows) > 0 {
			s += "<div class='" + App.Proj.Html.ClsPanelRows + "'>"
			for i := range panel.SubRows {
				s += "<div class='" + App.Proj.Html.ClsPanelRow + "'>" + iter(sheetVer, &panel.SubRows[i]) + "</div>"
			}
			s += "</div>"
		} else if len(panel.SubCols) > 0 {
			s += "<div class='" + App.Proj.Html.ClsPanelCols + "'>"
			for i := range panel.SubCols {
				sc := &panel.SubCols[i]
				pw := sc.Rect.Max.X - sc.Rect.Min.X
				sw := sheetVer.meta.PanelsTree.Rect.Max.X - sheetVer.meta.PanelsTree.Rect.Min.X
				pp := int(99.0 / (float64(sw) / float64(pw)))
				s += "<div class='" + App.Proj.Html.ClsPanelCol + "' style='width: " + itoa(pp) + "%'>" + iter(sheetVer, sc) + "</div>"
			}
			s += "</div>"
		} else {
			langid := langId
			if !panel.HasAny(langid) {
				langid = App.Proj.Langs[0].Name
			}
			name := strings.ToLower(App.Proj.meta.ContentHashes[sheetVer.fileName] + langid + itoa(quali.SizeHint) + itoa(pidx))
			s += "<div class='" + App.Proj.Html.ClsPanel + "'><img alt='" + name + "' title='" + name + "' src='./img/" + name + ".svg'/></div>"
			pidx++
		}
		return
	}
	for _, sheet := range sheets {
		assert(len(sheet.versions) == 1)
		sheetver := sheet.versions[0]
		sheetver.ensure(true)
		pidx = 0
		page.PageContent += "<div class='tsheet'>"
		page.PageContent += iter(sheetver, sheetver.meta.PanelsTree)
		page.PageContent += "</div>"
	}
}
