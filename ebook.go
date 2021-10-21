package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type BookConfig struct {
	KeepUntranslated bool
}

type Book struct {
	Config   string
	Name     string
	Title    map[string]string
	Chapters []struct {
		FromSeries                     []string
		ExcludeBySeriesAndChapterNames map[string][]string
		ExcludeBySheetName             []string
		RewriteToMonths                bool
	}

	config     *BookConfig
	parentProj *Project
}

func (me *Book) ToSeries() *Series {
	var series = &Series{
		Book:       me,
		Name:       me.Name,
		UrlName:    me.Name,
		Title:      me.Title,
		parentProj: me.parentProj,
	}

	for _, chapspec := range me.Chapters {
		var srcchaps []*Chapter
		if len(chapspec.FromSeries) == 0 {
			for _, series := range me.parentProj.Series {
				chapspec.FromSeries = append(chapspec.FromSeries, series.Name)
			}
		}
		for _, seriesname := range chapspec.FromSeries {
			series := me.parentProj.seriesByName(seriesname)
			if series == nil {
				panic("No such series: " + seriesname)
			}
			for _, chapter := range series.Chapters {
				var excluded bool
				if chapspec.ExcludeBySeriesAndChapterNames != nil {
					for _, excludename := range chapspec.ExcludeBySeriesAndChapterNames[seriesname] {
						if excluded = (excludename == chapter.Name); excluded {
							break
						}
					}
				}
				if !excluded {
					srcchaps = append(srcchaps, chapter)
				}
			}
		}

		var newchaps []*Chapter
		for _, srcchap := range srcchaps {
			var newchap = &Chapter{
				Name:  srcchap.Name,
				Title: srcchap.Title,
			}
			for _, sheet := range srcchap.sheets {
				var excluded bool
				for _, excludename := range chapspec.ExcludeBySheetName {
					if excluded = (excludename == sheet.name); excluded {
						break
					}
				}
				if excluded {
					continue
				}

				newchap.sheets = append(newchap.sheets, &Sheet{
					name:          sheet.name,
					parentChapter: newchap,
					versions:      []*SheetVer{sheet.versions[0]},
				})
			}
			newchaps = append(newchaps, newchap)
		}
		if chapspec.RewriteToMonths {
			newchaps = me.rewriteToMonths(newchaps)
		}
		for _, newchap := range newchaps {
			newchap.UrlName = newchap.Name
			newchap.SheetsPerPage = 1
			newchap.parentSeries = series
			newchap.versions = []int64{0}
			if len(newchap.Title) == 0 {
				newchap.Title = map[string]string{me.parentProj.Langs[0]: newchap.Name}
			}
			for _, sheet := range newchap.sheets {
				sv := sheet.versions[0]
				if newchap.verDtLatest.from <= 0 || sv.dateTimeUnixNano < newchap.verDtLatest.from {
					newchap.verDtLatest.from = sv.dateTimeUnixNano
				}
				if newchap.verDtLatest.until <= 0 || sv.dateTimeUnixNano > newchap.verDtLatest.until {
					newchap.verDtLatest.until = sv.dateTimeUnixNano
				}
			}
		}
		series.Chapters = append(series.Chapters, newchaps...)
	}

	return series
}

func (me *Book) rewriteToMonths(chaps []*Chapter) []*Chapter {
	var allsheets []*Sheet
	var monthchaps []*Chapter

	sheetidsdone := map[string]bool{}
	for _, chap := range chaps {
		for _, sheet := range chap.sheets {
			if sv := sheet.versions[0]; !sheetidsdone[sv.id] {
				sheetidsdone[sv.id] = true
				allsheets = append(allsheets, sheet)
			}
		}
	}
	sort.SliceStable(allsheets, func(i int, j int) bool {
		return allsheets[i].versions[0].dateTimeUnixNano < allsheets[j].versions[0].dateTimeUnixNano
	})
	for _, sheet := range allsheets {
		dt := time.Unix(0, sheet.versions[0].dateTimeUnixNano)
		chapname := strconv.Itoa(dt.Year()) + "-" + strconv.Itoa(int(dt.Month()))
		var chap *Chapter
		for _, monthchap := range monthchaps {
			if monthchap.Name == chapname {
				chap = monthchap
				break
			}
		}
		if chap == nil {
			monthname, yearname := dt.Month().String(), strconv.Itoa(dt.Year())
			chap = &Chapter{Name: chapname,
				Title: map[string]string{me.parentProj.Langs[0]: monthname + " " + yearname}}
			for _, lang := range me.parentProj.Langs[1:] {
				if s := me.parentProj.PageContentTexts[lang]["Month_"+monthname]; s != "" {
					chap.Title[lang] = s + " " + yearname
				}
			}
			monthchaps = append(monthchaps, chap)
		}
		chap.sheets = append(chap.sheets, sheet)
	}
	return monthchaps
}

func (me *Series) genBook(sg *siteGen, outDirPath string, qIdx int, lang string, bgCol bool, dirRtl bool, onDone func()) {
	defer onDone()
	proj, quali, book := me.parentProj, me.parentProj.Qualis[qIdx], me.Book

	bookid := me.UrlName + "_" + lang + strIf(bgCol, "_col_", "_bw_") + strIf(dirRtl, proj.DirModes.Rtl.Name, proj.DirModes.Ltr.Name) + "_" + strconv.Itoa(quali.SizeHint)
	outfilepath := filepath.Join(outDirPath, bookid+".epub")
	_ = os.Remove(outfilepath)
	outfile, err := os.Create(outfilepath)
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	zw := zip.NewWriter(outfile)

	files := []struct {
		Path string
		Data string
	}{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0" encoding="UTF-8" ?>
			<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
			<rootfiles>
				<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
			</rootfiles>
			</container>`},
		{"OEBPS/nav.xhtml", `<?xml version="1.0" encoding="UTF-8" ?>
			<nav epub:type="toc" id="toc">
				<h2>MuhTOC</h2>
				<ol>
				<li>
					<a href="chapter1.html">LeChap1</a>
				</li>
				</ol>
			</nav>`},
		{"OEBPS/content.opf", `<?xml version="1.0" encoding="UTF-8" ?>
			<package version="3.0" dir="` + strIf(dirRtl, "rtl", "ltr") + `" xml:lang="` + lang + `" xmlns="http://www.idpf.org/2007/opf" unique-identifier="` + bookid + `">
				<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
					<dc:title>` + locStr(book.Title, lang) + `</dc:title>
					<dc:language>` + lang + `</dc:language>
					<dc:identifier>` + bookid + `</dc:identifier>
					<meta property="dcterms:modified">` + time.Now().Format("2006-01-02T15:04:05Z") + `</meta>
				</metadata>
				<manifest>
					<item href="chapter1.html" id="chap1" media-type="text/html"></item>
				</manifest>
				<spine>
					<itemref idref="chap1"></itemref>
				</spine>
			</package>`},
		{"OEBPS/chapter1.html", "<!DOCTYPE html><html><head><title>foobar</title></head><body>Hello World<hr/>" + bookid + "</body></html>"},
	}
	for _, file := range files {
		f, err := zw.Create(file.Path)
		if err != nil {
			panic(err)
		}
		if _, err = f.Write([]byte(file.Data)); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	_ = outfile.Sync()
	printLn(outfilepath)
}
