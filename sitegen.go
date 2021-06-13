package main

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
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
	if err := os.RemoveAll(".build"); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	mkDir(".build")

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

	tmpl, err := template.New("index.html").ParseFiles("_sitetmpl/index.html")
	if err != nil {
		panic(err)
	}

	siteGenPage(tmpl, nil, nil, nil, nil)
}

func siteGenPage(tmpl *template.Template, series *Series, chapter *Chapter, sheet *Sheet, sheetVer *SheetVer) {
	name, page := "index", PageGen{SiteTitle: App.Proj.Title, SiteDesc: App.Proj.Desc}

	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, name+".html", &page); err != nil {
		panic(err)
	}
	writeFile(".build/"+name+".html", buf.Bytes())
}
