package main

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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

	config  *BookConfig
	genPrep struct {
		files      map[string]map[string]string
		imgDirPath string
		svgs       map[*SheetVer]string
		pngs       map[int]map[*SheetVer]string
	}
}

func (me *Book) id(lang string, bgCol bool, dirRtl bool, qIdx int) string {
	return me.Name + "_" + lang + strIf(bgCol, "_col_", "_bw_") +
		strIf(dirRtl, App.Proj.DirModes.Rtl.Name, App.Proj.DirModes.Ltr.Name) +
		"_" + strconv.Itoa(App.Proj.Qualis[qIdx].SizeHint)
}

func (me *Book) toSeries() *Series {
	var series = &Series{
		Book:    me,
		Name:    me.Name,
		UrlName: me.Name,
		Title:   me.Title,
	}

	for _, chapspec := range me.Chapters {
		var srcchaps []*Chapter
		if len(chapspec.FromSeries) == 0 {
			for _, series := range App.Proj.Series {
				chapspec.FromSeries = append(chapspec.FromSeries, series.Name)
			}
		}
		for _, seriesname := range chapspec.FromSeries {
			series := App.Proj.seriesByName(seriesname)
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
				newchap.Title = map[string]string{App.Proj.Langs[0]: newchap.Name}
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
				Title: map[string]string{App.Proj.Langs[0]: monthname + " " + yearname}}
			for _, lang := range App.Proj.Langs[1:] {
				if s := App.Proj.PageContentTexts[lang]["Month_"+monthname]; s != "" {
					chap.Title[lang] = s + " " + yearname
				}
			}
			monthchaps = append(monthchaps, chap)
		}
		chap.sheets = append(chap.sheets, sheet)
	}
	return monthchaps
}

func (me *Series) genBookPrep(sg *siteGen) {
	book := me.Book

	book.genPrep.files = map[string]map[string]string{}
	for _, lang := range App.Proj.Langs {
		book.genPrep.files[lang] = map[string]string{}

		book.genPrep.files[lang]["OEBPS/nav.xhtml"] = `<?xml version="1.0" encoding="UTF-8" ?>
			<html xmlns="http://www.w3.org/1999/xhtml"><head></head><body><nav xmlns:epub="http://www.idpf.org/2007/ops" epub:type="toc" id="toc">
				<ol>
				<li>
					<a href="chapter1.xhtml">LeChap1</a>
				</li>
				</ol>
			</nav></body></html>`

		book.genPrep.files[lang]["OEBPS/content.opf"] = `<?xml version="1.0" encoding="UTF-8" ?>
			<package version="3.0" dir="$DIR" xml:lang="` + lang + `" xmlns="http://www.idpf.org/2007/opf" unique-identifier="bid">
				<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
					<dc:identifier id="bid">urn:uuid:$BOOKID</dc:identifier>
					<dc:title>` + locStr(book.Title, lang) + `</dc:title>
					<dc:language>` + lang + `</dc:language>
					<meta property="dcterms:modified">` + time.Now().Format("2006-01-02T15:04:05Z") + `</meta>
				</metadata>
				<manifest>
					<item href="nav.xhtml" id="nav" properties="nav" media-type="application/xhtml+xml"></item>
					<item href="chapter1.xhtml" id="chap1" media-type="application/xhtml+xml"></item>
				</manifest>
				<spine>
					<itemref idref="chap1"></itemref>
				</spine>
			</package>`

		book.genPrep.files[lang]["OEBPS/chapter1.xhtml"] = `<?xml version="1.0" encoding="UTF-8" ?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>foobar</title></head><body>Hello World</body></html>`
	}

	book.genPrep.svgs = map[*SheetVer]string{}
	book.genPrep.pngs = map[int]map[*SheetVer]string{}
	for i, q := range App.Proj.Qualis {
		if q.UseForBooks {
			book.genPrep.pngs[i] = map[*SheetVer]string{}
		}
	}
	book.genPrep.imgDirPath = "/dev/shm/" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var work sync.WaitGroup
	var lock sync.Mutex
	mkDir(book.genPrep.imgDirPath)
	for _, dirRtl := range []bool{false, true} {
		for _, bgCol := range []bool{false, true} {
			for _, lang := range App.Proj.Langs {
				for _, chap := range me.Chapters {
					for _, sheet := range chap.sheets {
						work.Add(1)
						sv := sheet.versions[0]
						go me.genBookSheetSvgAndPngs(sv, filepath.Join(book.genPrep.imgDirPath, sheet.name+strIf(dirRtl, "_rtl_", "_ltr_")+lang+strIf(bgCol, "_col", "_bw")+".svg"), dirRtl, lang, bgCol, work.Done, &lock)
					}
				}
			}
		}
	}
	work.Wait()
}

func (me *Series) genBookSheetSvgAndPngs(sv *SheetVer, outFilePath string, dirRtl bool, lang string, bgCol bool, onDone func(), lock *sync.Mutex) {
	defer onDone()
	if bgCol && !sv.data.hasBgCol {
		return
	}
	w, h := sv.data.PanelsTree.Rect.Max.X, sv.data.PanelsTree.Rect.Max.Y
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
		xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
		width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">
			<style type="text/css">
				polygon { stroke: black; fill: white; }
				@font-face { ` +
		strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) + ` }
				g > svg > svg > text, g > svg > svg > text > tspan { ` +
		strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + ` }
			</style>`

	pidx := 0

	sv.data.PanelsTree.iter(func(p *ImgPanel) {
		panelpngsrcfilepath, err := filepath.Abs(filepath.Join(sv.data.PicDirPath(App.Proj.Qualis[App.Proj.maxQualiIdx()].SizeHint), itoa(pidx)+".png"))
		if err != nil {
			panic(err)
		}

		x, y, w, h := p.Rect.Min.X, p.Rect.Min.Y, p.Rect.Dx(), p.Rect.Dy()
		gid := "pnl" + itoa(pidx)
		svg += `<g id="` + gid + `" transform="translate(` + itoa(x) + ` ` + itoa(y) + `)">`
		if bgCol {
			panelbgpngsrcfilepath, err := filepath.Abs(filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".png"))
			if err != nil {
				panic(err)
			}
			svg += `<image x="0" y="0" width="` + itoa(w) + `" height="` + itoa(h) + `"
			xlink:href="` + panelbgpngsrcfilepath + `" />`
		} else {
			svg += `<rect x="0" y="0" stroke="#000000" stroke-width="0" fill="#ffffff"
				width="` + itoa(w) + `" height="` + itoa(h) + `"></rect>`
		}
		svg += `<image x="0" y="0" width="` + itoa(w) + `" height="` + itoa(h) + `"
				xlink:href="` + panelpngsrcfilepath + `" />`
		svg += sv.genTextSvgForPanel(pidx, p, lang, false)
		svg += "\n</g>\n\n"
		pidx++
	})
	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
	lock.Lock()
	me.Book.genPrep.svgs[sv] = outFilePath
	lock.Unlock()

	var work sync.WaitGroup
	for qidx := range App.Proj.Qualis {
		if App.Proj.Qualis[qidx].UseForBooks {
			work.Add(1)
			go func(qidx int) {
				defer work.Done()
				pngfilepath := outFilePath + "." + itoa(App.Proj.Qualis[qidx].SizeHint) + ".png"
				cmdargs := []string{outFilePath, "-quality", "90"}
				if qidx != App.Proj.maxQualiIdx() {
					cmdargs = append(cmdargs, "-resize", itoa(App.Proj.Qualis[qidx].SizeHint))
				}
				cmd := exec.Command("convert", append(cmdargs, pngfilepath)...)
				if output, err := cmd.CombinedOutput(); err != nil {
					panic(err)
				} else if s := strings.TrimSpace(string(output)); s != "" {
					panic(s)
				} else {
					lock.Lock()
					me.Book.genPrep.pngs[qidx][sv] = pngfilepath
					lock.Unlock()
				}
			}(qidx)
		}
	}
	work.Wait()
}

func (me *Series) genBookBuild(sg *siteGen, outDirPath string, qIdx int, lang string, bgCol bool, dirRtl bool, onDone func()) {
	defer onDone()
	bookid := me.Book.id(lang, bgCol, dirRtl, qIdx)
	me.genBookBuildEpub(bookid, filepath.Join(outDirPath, bookid+".epub"), qIdx, lang, bgCol, dirRtl)
}

func (me *Series) genBookBuildEpub(bookId string, outFilePath string, qIdx int, lang string, bgCol bool, dirRtl bool) {
	bookId = strToUuidLike(bookId) // not so uu really
	_ = os.Remove(outFilePath)
	outfile, err := os.Create(outFilePath)
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
	}
	repl := strings.NewReplacer("$DIR", strIf(dirRtl, "rtl", "ltr"), "$BOOKID", bookId)
	for prepFilePath, prepFileData := range me.Book.genPrep.files[lang] {
		files = append(files, struct {
			Path string
			Data string
		}{prepFilePath, repl.Replace(prepFileData)})
	}
	for i, file := range files {
		var f io.Writer
		var err error
		if i == 0 {
			f, err = zw.CreateRaw(&zip.FileHeader{Method: zip.Store, NonUTF8: true, Name: file.Path})
		} else {
			f, err = zw.Create(file.Path)
		}
		if err != nil {
			panic(err)
		} else if _, err = f.Write([]byte(file.Data)); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	_ = outfile.Sync()
}
