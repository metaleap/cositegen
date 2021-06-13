package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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
	siteGenPages(tmpl, nil, nil, 0)
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			if chapter.SheetsPerPage > 0 {
				for i := 1; i <= (len(chapter.sheets) / chapter.SheetsPerPage); i++ {
					siteGenPages(tmpl, series, chapter, i)
				}
			} else {
				siteGenPages(tmpl, series, chapter, 0)
			}
		}
	}

	printLn("SiteGen: generating PNGs & SVGs...")
	time.Sleep(time.Minute)

	printLn("SiteGen: DONE!")
}

func siteGenPages(tmpl *template.Template, series *Series, chapter *Chapter, pageNumber int) {
	assert((series == nil) == (chapter == nil))

	name, page := "index", PageGen{SiteTitle: App.Proj.Title, SiteDesc: App.Proj.Desc}

	if series == nil && chapter == nil {
		siteGenPageExecAndWrite(tmpl, name, &page)
	} else {
		for _, lang := range App.Proj.languages {
			for _, quali := range []string{"hd" /*1280*/, "fhd" /*1920*/, "qhd4k" /*3840*/, "uhd8k" /*7680*/} {
				name = series.Name + "-" + chapter.Name + "-p" + itoa(pageNumber) + "-" + quali + "-" + lang[0]

				siteGenPageExecAndWrite(tmpl, name, &page)
			}
		}
	}
}

func siteGenPageExecAndWrite(tmpl *template.Template, name string, page *PageGen) {
	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, "_tmpl.html", page); err != nil {
		panic(err)
	}
	writeFile(".build/"+name+".html", buf.Bytes())
}
