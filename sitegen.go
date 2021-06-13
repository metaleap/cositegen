package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type PageGen struct {
	SiteTitle   string
	SiteDesc    string
	PageTitle   string
	PageDesc    string
	LangsList   string
	QualiList   string
	PageContent string
	FooterHtml  string
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

	printLn("SiteGen: generating PNGs & SVGs...")

	printLn("SiteGen: DONE!")
	browserCmd[len(browserCmd)-1] = "--app=file://" + os.Getenv("PWD") + "/.build/index.html"
	printLn(browserCmd)
	cmd := exec.Command(browserCmd[0], browserCmd[1:]...)
	if err := cmd.Run(); err != nil {
		printLn(err)
	}
}

func siteGenPages(tmpl *template.Template, series *Series, chapter *Chapter, langId string, pageNumber int) {
	assert((series == nil) == (chapter == nil))

	name, page := "index", PageGen{
		SiteTitle:   hEsc(App.Proj.Title),
		SiteDesc:    hEsc(App.Proj.Desc[langId]),
		PageTitle:   hEsc(siteGenTextStr("HomeTitle", langId)),
		PageDesc:    hEsc(siteGenTextStr("HomeDesc", langId)),
		PageContent: hEsc("Page contents..."),
		FooterHtml:  siteGenTextStr("FooterHtml", langId),
	}
	if page.SiteDesc == "" && langId != App.Proj.Langs[0].Name {
		page.SiteDesc = App.Proj.Desc[App.Proj.Langs[0].Name]
	}
	if langId != App.Proj.Langs[0].Name {
		name += "-" + langId
	}

	if series == nil && chapter == nil {
		siteGenPageExecAndWrite(tmpl, name, langId, &page)
	} else {
		for _, quali := range App.Proj.Qualis {
			name = series.Name + "-" + chapter.Name + "-" + quali.Name
			if pageNumber != 0 {
				name += "-p" + itoa(pageNumber)
			}
			name += "-" + langId

			page.QualiList = ""
			for _, q := range App.Proj.Qualis {
				href := strings.Replace(name, "-"+quali.Name+"-", "-"+q.Name+"-", 1)
				page.QualiList += "<option value='" + strings.ToLower(href) + "'"
				if q.Name == quali.Name {
					page.QualiList += " selected='selected'"
				}
				page.QualiList += ">" + q.Name + "</option>"
			}
			page.QualiList = "<select id='qualilist'>" + page.QualiList + "</select>"

			siteGenPageExecAndWrite(tmpl, name, langId, &page)
		}
	}
}

func siteGenPageExecAndWrite(tmpl *template.Template, name string, langId string, page *PageGen) {
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

func siteGenTextStr(key string, langId string) (s string) {
	if s = App.Proj.PageContentTexts[langId][key]; s == "" {
		if s = App.Proj.PageContentTexts[App.Proj.Langs[0].Name][key]; s == "" {
			s = key
		}
	}
	return s
}
