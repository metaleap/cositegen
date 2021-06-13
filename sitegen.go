package main

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PageGen struct {
	SiteTitle   string
	SiteDesc    string
	PageTitle   string
	PageDesc    string
	PageContent string
}

func siteGen() {
	defer func() {
		if err := recover(); err != nil {
			printLn("SiteGen Error: ", err)
		}
	}()
	printLn("SiteGen started. When done, result will open in new window.")

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
	tmpl, err := template.New("index.html").ParseFiles("_sitetmpl/index.html")
	if err != nil {
		panic(err)
	}
	siteGenPage(tmpl, nil, nil, nil, nil)
	for _, series := range App.Proj.Series {
		siteGenPage(tmpl, series, nil, nil, nil)
		for _, chapter := range series.Chapters {
			siteGenPage(tmpl, series, chapter, nil, nil)
		}
	}

	printLn("SiteGen: generating PNGs & SVGs...")
	time.Sleep(time.Minute)

	printLn("SiteGen: DONE!")
}

func siteGenPage(tmpl *template.Template, series *Series, chapter *Chapter, sheet *Sheet, sheetVer *SheetVer) {
	name, page := "index", PageGen{SiteTitle: App.Proj.Title, SiteDesc: App.Proj.Desc}

	if series != nil {
		name = series.Name
		if chapter != nil {
			name += "-" + chapter.Name
		}
	}

	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, "index.html", &page); err != nil {
		panic(err)
	}
	writeFile(".build/"+name+".html", buf.Bytes())
}
