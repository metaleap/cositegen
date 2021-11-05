package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Size struct {
	MmWidth     int
	MmHeight    int
	MmCutBorder int
}

func (me Size) px() Size         { return Size{me.pxWidth(), me.pxHeight(), me.pxCutBorder()} }
func (me Size) pxHeight() int    { return int(math.Ceil(float64(me.MmHeight) * (0.1 * dpi1200))) }
func (me Size) pxWidth() int     { return int(math.Ceil(float64(me.MmWidth) * (0.1 * dpi1200))) }
func (me Size) pxCutBorder() int { return int(math.Ceil(float64(me.MmCutBorder) * (0.1 * dpi1200))) }

type BookBuild struct {
	BasedOn                  string
	Config                   string
	Book                     string
	InclRtl                  bool
	InclBw                   bool
	NoLangs                  bool
	NoCol                    bool
	NoHiRes                  bool
	NoLoRes                  bool
	NoDirtPages              bool
	DropUntranslated         bool
	DoubleSpreadArrowsForLen uint64

	OverrideBook   BookDef
	OverrideConfig BookConfig
	OverrideFilter BookFilter

	name                  string
	genPrepDirPath        string
	genNumUniqueDirtPages int
	config                BookConfig
	book                  BookDef
	series                *Series
}

type BookConfig struct {
	PageSize        Size
	CoverSize       Size
	PxLoResWidth    int
	MinPageCount    int
	DecosFromSeries string
	OffsetsMm       struct {
		CoverGap int
		Small    int
		Large    int
	}
}

type BookFilter struct {
	ExcludeBySeriesAndChapterNames map[string][]string
	ExcludeBySheetName             []string
	OnlySheetsNamed                []string
}

type BookDef struct {
	Title    map[string]string
	Desc     map[string]string
	Chapters []struct {
		FromSeries        []string
		Filter            BookFilter
		ReChapterToMonths bool
		SwapSheets        map[string]string
	}
	CssTitle           string
	CssToc             string
	CssDesc            string
	CssPgNr            string
	CssCoverTitle      string
	CssCoverSpine      string
	CoverTitlePxOffset int

	name string
}

func (me *BookBuild) mergeOverrides() {
	if base := App.Proj.BookBuilds[me.BasedOn]; base != nil && base != me {
		if me.Config == "" {
			me.config = base.config
		}
		if me.Book == "" {
			me.book = base.book
		}
		if !me.InclBw {
			me.InclBw = base.InclBw
		}
		if !me.InclRtl {
			me.InclRtl = base.InclRtl
		}
		if !me.NoHiRes {
			me.NoHiRes = base.NoHiRes
		}
		if (!me.NoLoRes) && !me.NoHiRes {
			me.NoLoRes = base.NoLoRes
		}
		if !me.NoLangs {
			me.NoLangs = base.NoLangs
		}
		if !me.NoCol {
			me.NoCol = base.NoCol
		}
		if !me.NoDirtPages {
			me.NoDirtPages = base.NoDirtPages
		}
		if !me.DropUntranslated {
			me.DropUntranslated = base.DropUntranslated
		}
		if me.DoubleSpreadArrowsForLen == 0 {
			me.DoubleSpreadArrowsForLen = base.DoubleSpreadArrowsForLen
		}
	}

	if len(me.OverrideBook.Chapters) != 0 {
		me.book.Chapters = me.OverrideBook.Chapters
	}
	if len(me.OverrideBook.CssPgNr) != 0 {
		me.book.CssPgNr = me.OverrideBook.CssPgNr
	}
	if len(me.OverrideBook.CssTitle) != 0 {
		me.book.CssTitle = me.OverrideBook.CssTitle
	}
	if len(me.OverrideBook.CssToc) != 0 {
		me.book.CssToc = me.OverrideBook.CssToc
	}
	if len(me.OverrideBook.Title) != 0 {
		me.book.Title = me.OverrideBook.Title
	}
	for i := range me.book.Chapters {
		if len(me.OverrideFilter.ExcludeBySeriesAndChapterNames) != 0 {
			me.book.Chapters[i].Filter.ExcludeBySeriesAndChapterNames = me.OverrideFilter.ExcludeBySeriesAndChapterNames
		}
		if len(me.OverrideFilter.ExcludeBySheetName) != 0 {
			me.book.Chapters[i].Filter.ExcludeBySheetName = me.OverrideFilter.ExcludeBySheetName
		}
		if len(me.OverrideFilter.OnlySheetsNamed) != 0 {
			me.book.Chapters[i].Filter.OnlySheetsNamed = me.OverrideFilter.OnlySheetsNamed
		}
	}
	if len(me.OverrideConfig.DecosFromSeries) != 0 {
		me.config.DecosFromSeries = me.OverrideConfig.DecosFromSeries
	}
	if me.OverrideConfig.MinPageCount > 0 {
		me.config.MinPageCount = me.OverrideConfig.MinPageCount
	}
	if me.OverrideConfig.PxLoResWidth > 0 {
		me.config.PxLoResWidth = me.OverrideConfig.PxLoResWidth
	}
	if me.OverrideConfig.CoverSize.MmHeight != 0 && me.OverrideConfig.CoverSize.MmWidth != 0 {
		me.config.CoverSize = me.OverrideConfig.CoverSize
	}
	if me.OverrideConfig.PageSize.MmHeight != 0 && me.OverrideConfig.PageSize.MmWidth != 0 {
		me.config.PageSize = me.OverrideConfig.PageSize
	}
}

func (me *BookBuild) id(lang string, bgCol bool, dirRtl bool, loRes bool) string {
	return me.name + strIf(bgCol, "_col_", "_bw_") + lang + strIf(loRes, "_lo_", "_hi_") +
		strIf(dirRtl, "rtl", "ltr")
}

func (me *BookDef) toSeries() (ret *Series) {
	ret = &Series{
		Book:    me,
		Name:    me.name,
		UrlName: me.name,
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
				if chapspec.Filter.ExcludeBySeriesAndChapterNames != nil {
					for _, exclname := range chapspec.Filter.ExcludeBySeriesAndChapterNames[seriesname] {
						if excluded = (exclname == chapter.Name); excluded {
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
				for _, exclname := range chapspec.Filter.ExcludeBySheetName {
					if excluded = (exclname == sheet.name); excluded {
						break
					}
				}
				if len(chapspec.Filter.OnlySheetsNamed) != 0 && !excluded {
					excluded = true
					for _, inclname := range chapspec.Filter.OnlySheetsNamed {
						if inclname == sheet.name {
							excluded = false
							break
						}
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
		if chapspec.ReChapterToMonths {
			newchaps = me.reChapterToMonths(newchaps)
		}
		for _, newchap := range newchaps {
			newchap.UrlName = newchap.Name
			newchap.SheetsPerPage = 1
			newchap.parentSeries = ret
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
			for swap1, swap2 := range chapspec.SwapSheets {
				i1, i2 := -1, -1
				for i, sheet := range newchap.sheets {
					if sheet.name == swap1 {
						i1 = i
					} else if sheet.name == swap2 {
						i2 = i
					} else if i1 >= 0 && i2 >= 0 {
						break
					}
				}
				if i1 >= 0 && i2 >= 0 {
					newchap.sheets[i1], newchap.sheets[i2] = newchap.sheets[i2], newchap.sheets[i1]
				}
			}
		}
		ret.Chapters = append(ret.Chapters, newchaps...)
	}
	return ret
}

func (me *BookDef) reChapterToMonths(chaps []*Chapter) []*Chapter {
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
				if s := App.Proj.textStr(lang, "Month_"+monthname); s != "" {
					chap.Title[lang] = s + " " + yearname
				}
			}
			monthchaps = append(monthchaps, chap)
		}
		chap.sheets = append(chap.sheets, sheet)
	}
	return monthchaps
}

func (me *BookBuild) genBookPrep(sg *siteGen, onDone func()) {
	if onDone != nil {
		defer onDone()
	}
	config, series := &me.config, me.series
	me.genPrepDirPath = "/dev/shm/" + strconv.FormatInt(time.Now().UnixNano(), 36)
	mkDir(me.genPrepDirPath)
	me.genBookDirtPageSvgs()
	panic("YO")
	var sheetsvgfilepaths, pagesvgfilepaths []string
	for lidx, lang := range App.Proj.Langs {
		if lidx > 0 && me.NoLangs {
			continue
		}
		pgnrs := map[*Chapter]int{}
		for _, bgcol := range []bool{false, true} {
			if bgcol && me.NoCol {
				continue
			}
			for _, dirrtl := range []bool{false, true} {
				if dirrtl && !me.InclRtl {
					continue
				}
				pgnr := 6
				for _, chap := range series.Chapters {
					pgnrs[chap] = pgnr
					for _, sheet := range chap.sheets {
						sv := sheet.versions[0]
						if skip := (me.DropUntranslated && lang != App.Proj.Langs[0] && App.Proj.percentTranslated(lang, series, chap, sv, -1) < 50); skip ||
							(bgcol && !sv.data.hasBgCol) {
							if !skip {
								pgnr++
							}
							continue
						}
						svgfilename := sheet.name + strIf(dirrtl, "_rtl_", "_ltr_") + lang + strIf(bgcol, "_col", "_bw") + ".svg"
						svgfilepath := filepath.Join(me.genPrepDirPath, svgfilename)
						sheetsvgfilepaths = append(sheetsvgfilepaths, svgfilepath)
						me.genBookSheetSvg(sv, svgfilepath, dirrtl, lang, bgcol)
						pagesvgfilename := "p" + itoa0(pgnr, 3) + strIf(dirrtl, "_rtl_", "_ltr_") + lang + strIf(bgcol, "_col", "_bw") + ".svg"
						pagesvgfilepath := filepath.Join(me.genPrepDirPath, pagesvgfilename)
						pagesvgfilepaths = append(pagesvgfilepaths, pagesvgfilepath)
						var doublearrow bool
						if strnumsuff := strNumericSuffix(sheet.name); me.DoubleSpreadArrowsForLen > 1 {
							ui, _ := strconv.ParseUint(strnumsuff, 10, 64)
							doublearrow = ui > 0 && ui < me.DoubleSpreadArrowsForLen
						}
						me.genBookSheetPageSvg(pagesvgfilepath, svgfilepath+".png", sv.data.pxBounds().Size(), pgnr, doublearrow)
						pgnr++
					}
				}
			}
		}
		svgfilepath := filepath.Join(me.genPrepDirPath, "p003_"+lang+".svg")
		pagesvgfilepaths = append(pagesvgfilepaths, svgfilepath)
		me.genBookTiTocPageSvg(svgfilepath, lang, pgnrs)
	}
	if !me.NoDirtPages {
		pagesvgfilepaths = append(pagesvgfilepaths, me.genBookDirtPageSvgs()...)
	}
	var work sync.WaitGroup
	work.Add(1)
	go me.genBookTitlePanelCutoutsPng(filepath.Join(me.genPrepDirPath, "cover.png"), &config.CoverSize, 0, config.OffsetsMm.CoverGap, work.Done)
	work.Add(1)
	go me.genBookTitlePanelCutoutsPng(filepath.Join(me.genPrepDirPath, "bgtoc.png"), &config.PageSize, 177, 0, work.Done)
	work.Wait()

	mkDir(".ccache/.svgpng")
	for i, svgfilepath := range sheetsvgfilepaths {
		if lrw := config.PxLoResWidth; lrw > 0 && me.NoHiRes {
			imgSvgToPng(svgfilepath, svgfilepath+".png", nil, lrw, 0, false, nil)
		} else {
			imgSvgToPng(svgfilepath, svgfilepath+".png", nil, 0, 1200, false, nil)
		}
		printLn("\t\t", time.Now().Format("15:04:05"), "shsvg", i+1, "/", len(sheetsvgfilepaths))
	}
	repl := strings.NewReplacer("./", strings.TrimSuffix(me.genPrepDirPath, "/")+"/")
	for i, svgfilepath := range pagesvgfilepaths {
		var work sync.WaitGroup
		if !me.NoHiRes {
			work.Add(1)
			go imgSvgToPng(svgfilepath, svgfilepath+".png", repl, 0, 1200, false, work.Done)
		}
		if lrw := config.PxLoResWidth; lrw > 0 && !me.NoLoRes {
			work.Add(1)
			go imgSvgToPng(svgfilepath, svgfilepath+"."+itoa(lrw)+".png", repl, lrw, 0, false, work.Done)
		}
		work.Wait()
		printLn("\t\t", time.Now().Format("15:04:05"), "pgsvg", i+1, "/", len(pagesvgfilepaths))
	}
}

func (me *BookBuild) genBookSheetPageSvg(outFilePath string, sheetImgFilePath string, sheetImgSize image.Point, pgNr int, doubleSpreadArrowIndicator bool) {
	book, config := &me.book, &me.config
	w, h, mm1 := config.PageSize.pxWidth(), config.PageSize.pxHeight(), config.PageSize.pxHeight()/config.PageSize.MmHeight
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
		xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
		width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">
		<style type="text/css">
			@font-face { ` + strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) + ` }
			text { ` + strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + "; " + book.CssPgNr + ` }
		</style>`

	// for now treat page as being odd-numbered aka. right-hand-side
	pgx := config.PageSize.MmWidth - (6 + config.OffsetsMm.Small)
	var mmx, mmy, mmw, mmh float64
	for mmo := float64(config.OffsetsMm.Large); mmy < float64(config.OffsetsMm.Small); mmo++ {
		mmx, mmw = mmo, float64(config.PageSize.MmWidth)-(float64(config.OffsetsMm.Small)+mmo)
		mmh = float64(mmw) / (float64(sheetImgSize.X) / float64(sheetImgSize.Y))
		mmy = 0.5 * (float64(config.PageSize.MmHeight) - mmh)
	}
	// but is it an even-numbered/left-hand-side page?
	if (pgNr % 2) == 0 {
		mmx, pgx = float64(config.OffsetsMm.Small), config.OffsetsMm.Small
	}

	svg += `<image x="` + itoa(int(float64(mm1)*mmx)) + `" y="` + itoa(int(float64(mm1)*mmy)) + `"
		width="` + itoa(int(float64(mm1)*mmw)) + `" height="` + itoa(int(float64(mm1)*mmh)) + `"
		xlink:href="./` + filepath.Base(sheetImgFilePath) + `" dx="0" dy="0" />`

	pgy := itoa(config.PageSize.pxHeight() - config.OffsetsMm.Small*mm1)
	svg += `<text dx="0" dy="0" x="` + itoa(pgx*mm1) + `" y="` + pgy + `">` + itoa0(pgNr, 3) + `</text>`
	if doubleSpreadArrowIndicator { // ➪
		svg += `<text dx="0" dy="0" x="` + itoa(w/2) + `" y="` + pgy + `">&#10155;</text>`
	}

	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *BookBuild) genBookTiTocPageSvg(outFilePath string, lang string, pgNrs map[*Chapter]int) {
	book, config, series := &me.book, &me.config, me.series
	w, h := config.PageSize.pxWidth(), config.PageSize.pxHeight()
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
	xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
	width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">`

	svg += `
			<style type="text/css">
			@font-face { ` +
		strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) + ` }
			.title, .toc, .desc, g > svg > svg > text, g > svg > svg > text > tspan { ` +
		strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + `; }
			.title { ` + book.CssTitle + ` }
			.toc { ` + book.CssToc + ` }
			.desc { ` + book.CssDesc + ` }
			</style>
			<image x="0" y="0" width="100%" height="100%" xlink:href="` + filepath.Join(me.genPrepDirPath, "bgtoc.png") + `" />`

	chapcount, pgnrlast := 0, 0
	for _, chap := range series.Chapters {
		pgnr := pgNrs[chap]
		if pgnr == pgnrlast {
			continue
		}
		pgnrlast, chapcount = pgnr, chapcount+1
	}

	textx, htoc, cc := h/9, 62.0/float64(chapcount), 0
	title, fullycal := book.Title, true
	for _, chap := range book.Chapters {
		if fullycal = chap.ReChapterToMonths; !fullycal {
			break
		}
	}
	if t := App.Proj.textStr(lang, "BookTocTitleCalendared"); fullycal && t != "" {
		title = map[string]string{lang: t}
	}
	svg += `<text class="title" x="` + itoa(textx) + `px" y="20%" dx="0" dy="0">` +
		htmlEscdToXmlEsc(hEsc(locStr(title, lang))) + `</text>`
	if len(book.Desc) != 0 {
		svg += `<text class="desc" x="` + itoa(textx) + `px" y="` + itoa(h-textx/3) + `px" dx="0" dy="0">` +
			htmlEscdToXmlEsc(hEsc(locStr(book.Desc, lang))) + `</text>`
	}

	pgnrlast = 0
	for _, chap := range series.Chapters {
		pgnr, texty := pgNrs[chap], int(26.0+(float64(cc)+1.0)*htoc)-5
		if pgnr == pgnrlast {
			continue
		}
		svg += `<text class="toc" x="` + itoa(textx*2) + `px" y="` + itoa(texty) + `%" dx="0" dy="0">` +
			htmlEscdToXmlEsc(hEsc(locStr(chap.Title, lang)+"······"+App.Proj.textStr(lang, "BookTocPagePrefix")+strIf(pgnr < 10, "0", "")+itoa(pgnr))) + `</text>`
		pgnrlast, cc = pgnr, cc+1
	}

	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *BookBuild) genBookDirtPageSvgs() (outFilePaths []string) {
	config, series := &me.config, me.series
	w, h, cb := config.PageSize.pxWidth(), config.PageSize.pxHeight(), config.PageSize.pxCutBorder()

	var svs []*SheetVer
	rand.Seed(time.Now().UnixNano())
	chaps := series.Chapters
	if forceFrom := App.Proj.seriesByName(config.DecosFromSeries); forceFrom != nil {
		chaps = forceFrom.Chapters
	}
	for _, chap := range chaps {
		for _, sheet := range chap.sheets {
			svs = append(svs, sheet.versions[0])
		}
	}
	rand.Shuffle(len(svs), func(i int, j int) {
		svs[i], svs[j] = svs[j], svs[i]
	})

	me.genNumUniqueDirtPages = 1 + (len(svs) / 16)
	perpage := float64(len(svs)) / float64(me.genNumUniqueDirtPages)
	perrowcol := int(math.Ceil(math.Sqrt(perpage)))

	for i := 0; i < me.genNumUniqueDirtPages; i++ {
		cw, ch := w/(perrowcol+1), h/(perrowcol+1)
		svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
					xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
					width="` + itoa(w+cb) + `" height="` + itoa(h+cb) + `"
					viewBox="0 0 ` + itoa(w+cb) + ` ` + itoa(h+cb) + `"><defs>`
		svs := svs[i*int(perrowcol*perrowcol):][:int(perrowcol*perrowcol)]
		for i, sv := range svs {
			svg += `<image xlink:href="` + absPath((sv.data.bwFilePath)) + `"
						id="p` + itoa(i) + `" width="` + itoa(cw) + `" height="` + itoa(ch) + `" />`
		}
		svg += `</defs><g opacity="0.44" transform="rotate(-15 ` + itoa(w/2) + ` ` + itoa(h/2) + `)">`
		var isv int
		for col := -1; col <= perrowcol+1; col++ {
			for row := -1; row <= perrowcol+1; row++ {
				cx, cy := col*cw, row*ch
				if (row % 2) == 0 {
					cx += cw / 2
				}
				svg += `<use x="` + itoa(cx) + `" y="` + itoa(cy) + `" xlink:href="#p` + itoa(isv) + `" />`
				isv = (isv + 1) % len(svs)
			}
		}

		svg += "</g></svg>"
		outfilepath := filepath.Join(me.genPrepDirPath, "dp"+itoa(i)+".svg")
		outFilePaths = append(outFilePaths, outfilepath)
		fileWrite(outfilepath, []byte(svg))
	}
	return
}

func (me *BookBuild) genBookTitlePanelCutoutsPng(outFilePath string, size *Size, inkColor uint8, mmCenterGap int, onDone func()) {
	if onDone != nil {
		defer onDone()
	}
	book, config, series := &me.book, &me.config, me.series

	var svs []*SheetVer
	rand.Seed(time.Now().UnixNano())
	chaps := series.Chapters
	if forceFrom := App.Proj.seriesByName(config.DecosFromSeries); forceFrom != nil {
		chaps = forceFrom.Chapters
	}
	for _, chap := range chaps {
		for _, sheet := range chap.sheets {
			if sv := sheet.versions[0]; sv.hasFaceAreas() {
				svs = append(svs, sv)
			}
		}
	}
	rand.Shuffle(len(svs), func(i int, j int) {
		svs[i], svs[j] = svs[j], svs[i]
	})

	faces := map[*image.Gray]image.Rectangle{}
	var work sync.WaitGroup
	var lock sync.Mutex
	for _, sv := range svs {
		work.Add(1)
		go func(sv *SheetVer) {
			defer work.Done()
			img, err := png.Decode(bytes.NewReader(fileRead(sv.data.bwFilePath)))
			if err != nil {
				panic(err)
			}
			imgpng := img.(*image.Gray)
			var pidx int
			sv.data.PanelsTree.iter(func(p *ImgPanel) {
				for _, area := range sv.panelFaceAreas(pidx) {
					subimg := imgpng.SubImage(area.Rect).(*image.Gray)
					fimg := subimg
					if inkColor != 0 {
						fimg = image.NewGray(area.Rect)
						for x := fimg.Bounds().Min.X; x < fimg.Bounds().Max.X; x++ {
							for y := fimg.Bounds().Min.Y; y < fimg.Bounds().Max.Y; y++ {
								gray := subimg.GrayAt(x, y)
								if gray.Y == 0 {
									gray.Y = inkColor
								} else if gray.Y != 255 {
									panic(gray.Y)
								}
								fimg.SetGray(x, y, gray)
							}
						}
					}
					lock.Lock()
					faces[fimg] = area.Rect
					lock.Unlock()
				}
				pidx++
			})
		}(sv)
	}
	work.Wait()

	for len(faces) < 4 {
		for img, rect := range faces {
			faces[img.SubImage(img.Bounds()).(*image.Gray)] = rect
		}
	}

	px1mm := float64(size.pxWidth()) / float64(size.MmWidth)
	numcols, numrows, numnope, wantall, pxcentergap := 0, 0, 0, mmCenterGap != 0, int(float64(mmCenterGap)*px1mm)
	{
		n, grids := len(faces), make([]int, 0, len(faces)/4)
		for _, min := range []int{2, 1} {
			for i := 1 + (n / 2); i > min; i-- {
				if d := n / i; d > min && i >= d {
					grids = append(grids, i)
				}
			}
			if len(grids) > 0 {
				break
			}
		}
		ratio := float64(size.pxWidth()-pxcentergap) / float64(size.pxHeight())
		sort.Slice(grids, func(i int, j int) bool {
			w1, h1 := grids[i], n/grids[i]
			w2, h2 := grids[j], n/grids[j]
			if mmCenterGap != 0 {
				if (w1%2) == 0 && (w2%2) != 0 {
					return true
				}
				if (w1%2) != 0 && (w2%2) == 0 {
					return false
				}
			}
			r1, r2 := float64(w1)/float64(h1), float64(w2)/float64(h2)
			d1, d2 := n-(w1*h1), n-(w2*h2)
			if r1 == r2 {
				return d1 < d2
			} else {
				return math.Max(ratio, r1)-math.Min(ratio, r1) < math.Max(ratio, r2)-math.Min(ratio, r2)
			}
		})
		numcols, numrows = grids[0], n/grids[0]
	}
	if diff := len(faces) - (numrows * numcols); wantall && diff > 0 {
		numrows += (1 + (diff / numcols))
		numnope = (numrows * numcols) - len(faces)
	}

	cellw, cellh := (size.pxWidth()-pxcentergap)/numcols, size.pxHeight()/numrows
	img := image.NewGray(image.Rect(0, 0, size.pxWidth(), size.pxHeight()))
	imgFill(img, image.Rect(0, 0, size.pxWidth(), size.pxHeight()), color.Gray{255})

	var fidx, cantitler int
	var cantitlel int
	numleftblank := 1
	for min := int(50.0 * px1mm); (numleftblank * cellw) < min; {
		numleftblank++
	}

	for fimg, frect := range faces {
		icol, irow, pad := fidx%numcols, fidx/numcols, size.pxHeight()/50
		if numnope > 0 && icol == 0 && irow == numrows-1 {
			cantitlel, icol, fidx = numleftblank*cellw, numleftblank, fidx+numleftblank
		}
		cx, cy, fw, fh := cellw*icol, cellh*irow, frect.Dx(), frect.Dy()
		if pxcentergap != 0 && icol >= (numcols/2) {
			cx += pxcentergap
		}
		sf := math.Min(float64(cellw-pad)/float64(fw), float64(cellh-pad)/float64(fh)) //scale factor
		dw, dh := int(float64(fw)*sf), int(float64(fh)*sf)
		dx, dy := cx+((cellw-dw)/2), cy+((cellh-dh)/2)
		drect := image.Rect(dx, dy, dx+dw, dy+dh)
		ImgScaler.Scale(img, drect, fimg, frect, draw.Over, nil)
		fidx++
		if cantitler = cx + cellw; cantitler >= size.pxWidth() {
			cantitler = 0
		}
	}
	if showhalfgap := false; showhalfgap {
		halfgapwidth := pxcentergap / 2
		halfgapleft := (size.pxWidth() / 2) - (halfgapwidth / 2)
		imgFill(img, image.Rect(halfgapleft, 0, halfgapleft+halfgapwidth, size.pxHeight()), color.Gray{64})
	}

	var buf bytes.Buffer
	if err := PngEncoder.Encode(&buf, img); err != nil {
		panic(err)
	}
	fileWrite(outFilePath, buf.Bytes())

	if mmCenterGap != 0 {
		for i, lang := range App.Proj.Langs {
			if i > 0 && me.NoLangs {
				continue
			}
			w, h, cb, title := size.pxWidth(), size.pxHeight(), size.pxCutBorder(), locStr(book.Title, lang)
			svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
				xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
				width="` + itoa(w+cb*2) + `" height="` + itoa(h+cb*2) + `" viewBox="0 0 ` + itoa(w+cb*2) + ` ` + itoa(h+cb*2) + `">`
			svg += `<image dx="0" dy="0" x="` + itoa(cb) + `" y="` + itoa(cb) + `" width="` + itoa(w) + `" height="` + itoa(h) + `"
						xlink:href="` + outFilePath + `" />`
			if cantitler != 0 {
				fs := 2.7 * (float64(w-cantitler) / float64(len(title)))
				if ch := float64(cellh); fs > ch {
					fs = ch
				}
				svg += `<rect stroke-width="0" fill="#000000" x="` + itoa(cb+cantitler) + `" y="` + itoa(cb+(numrows*cellh-cellh)) + `" width="` + itoa(w) + `" height="` + itoa(cellh*4) + `"/>
							<text dx="0" dy="0" x="` + itoa(cb+(cantitler-int(fs*0.22))) + `" y="` + itoa(cb+(h-(cellh/3))) + `" style="` + strings.Replace(book.CssCoverTitle, "font-size:", "-moz-moz:", -1) + `; font-size: ` + itoa(int(fs)) + `px !important">` + htmlEscdToXmlEsc(hEsc(title)) + `</text>`
			}
			svg += `<rect stroke-width="0" fill="#000000" x="` + itoa(cb+(w/2-pxcentergap/2)) + `" y="` + itoa(-h) + `" width="` + itoa(pxcentergap) + `" height="` + itoa(h*3) + `"/>
						<text dx="0" dy="0" x="` + itoa(cb+(w/2)) + `" y="` + itoa(cb+(h/4)) + `" style="` + book.CssCoverSpine + `">` + htmlEscdToXmlEsc(hEsc(title)) + `</text>`
			if cantitlel != 0 {
				fs := 2.7 * (float64(cantitlel) / float64(len(title)))
				svg += `<rect stroke-width="0" fill="#000000" x="` + itoa(-w) + `" y="` + itoa(cb+(numrows*cellh-cellh)) + `" width="` + itoa(w+cantitlel+cb) + `" height="` + itoa(cellh*4) + `"/>
							<text dx="0" dy="0" x="` + itoa(cb) + `" y="` + itoa(cb+(h-(cellh/3))) + `" style="` + strings.Replace(book.CssCoverTitle, "font-size:", "-moz-moz:", -1) + `; font-size: ` + itoa(int(fs)) + `px !important">` + htmlEscdToXmlEsc(hEsc(title)) + `</text>`
			}
			svg += `</svg>`
			fileWrite(outFilePath+"."+lang+".svg", []byte(svg))
		}
	}
}

func (*BookBuild) genBookSheetSvg(sv *SheetVer, outFilePath string, dirRtl bool, lang string, bgCol bool) {
	rectinner := sv.data.pxBounds()
	w, h := rectinner.Dx(), rectinner.Dy()

	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
		xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
		width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">
			<style type="text/css">
				polygon { stroke: black; fill: white; }
				@font-face { ` +
		strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) +
		` 		}
				g > svg > svg > text, g > svg > svg > text > tspan { ` +
		strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") +
		` 		}
			</style>`

	pidx := 0

	sv.data.PanelsTree.iter(func(p *ImgPanel) {
		px, py, pw, ph := p.Rect.Min.X-rectinner.Min.X, p.Rect.Min.Y-rectinner.Min.Y, p.Rect.Dx(), p.Rect.Dy()
		if px < 0 {
			panic(px)
		}
		if py < 0 {
			panic(py)
		}
		tx, gid := px, "pnl"+itoa(pidx)
		if dirRtl {
			tx = w - pw - px
		}
		svg += `<g id="` + gid + `" clip-path="url(#c` + gid + `)" transform="translate(` + itoa(tx) + ` ` + itoa(py) + `)">`
		svg += `<defs><clipPath id="c` + gid + `"><rect x="0" y="0"  width="` + itoa(pw) + `" height="` + itoa(ph) + `"></rect></clipPath></defs>`
		if panelbgpngsrcfilepath := absPath(filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".png")); bgCol && fileStat(panelbgpngsrcfilepath) != nil {
			svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
						xlink:href="` + panelbgpngsrcfilepath + `" />`
		} else {
			svg += `<rect x="0" y="0" stroke="#000000" stroke-width="0" fill="#ffffff"
						width="` + itoa(pw) + `" height="` + itoa(ph) + `"></rect>`
		}
		panelpngsrcfilepath := absPath(filepath.Join(sv.data.PicDirPath(App.Proj.Qualis[App.Proj.maxQualiIdx()].SizeHint), itoa(pidx)+".png"))
		svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
					xlink:href="` + panelpngsrcfilepath + `" />`
		svg += sv.genTextSvgForPanel(pidx, p, lang, false, true)
		svg += "\n</g>\n\n"
		pidx++
	})
	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *BookBuild) genBookBuild(outDirPath string, lang string, bgCol bool, dirRtl bool, loRes bool, onDone func()) {
	defer onDone()
	config, series := &me.config, me.series
	pgnr, idp, srcfilepaths := 1, 0, make([]string, 0, series.numSheets())

	for ; pgnr <= 5; pgnr++ {
		srcfilepath := filepath.Join(me.genPrepDirPath, "dp"+itoa(idp)+".svg"+strIf(loRes, "."+itoa(config.PxLoResWidth), "")+".png")
		if pgnr == 3 {
			srcfilepath = filepath.Join(me.genPrepDirPath, "p003_"+lang+".svg"+strIf(loRes, "."+itoa(config.PxLoResWidth), "")+".png")
		} else if me.genNumUniqueDirtPages != 0 {
			idp = (idp + 1) % me.genNumUniqueDirtPages
		}
		if pgnr == 3 || !me.NoDirtPages {
			srcfilepaths = append(srcfilepaths, srcfilepath)
		}
	}
	for _, chap := range series.Chapters {
		for _, sheet := range chap.sheets {
			sv := sheet.versions[0]
			if me.DropUntranslated && lang != App.Proj.Langs[0] && App.Proj.percentTranslated(lang, series, chap, sv, -1) < 50 {
				continue
			}
			srcfilepaths = append(srcfilepaths, filepath.Join(me.genPrepDirPath,
				"p"+itoa0(pgnr, 3)+strIf(dirRtl, "_rtl_", "_ltr_")+lang+strIf(bgCol && sv.data.hasBgCol, "_col", "_bw")+".svg"+strIf(loRes, "."+itoa(config.PxLoResWidth), "")+".png"))
			pgnr++
		}
	}
	if !me.NoDirtPages {
		for numtrailingempties := 0; !(numtrailingempties >= 4 && (len(srcfilepaths)%4) == 0 && len(srcfilepaths) >= config.MinPageCount); numtrailingempties++ {
			srcfilepaths = append(srcfilepaths, filepath.Join(me.genPrepDirPath, "dp"+itoa(idp)+".svg"+strIf(loRes, "."+itoa(config.PxLoResWidth), "")+".png"))
			idp = (idp + 1) % me.genNumUniqueDirtPages
		}
	}

	bookid := me.id(lang, bgCol, dirRtl, loRes)
	var work sync.WaitGroup
	work.Add(3)
	go imgSvgToPng(filepath.Join(me.genPrepDirPath, "cover.png."+lang+".svg"), filepath.Join(outDirPath, "cover."+lang+".png"), nil, 0, 1200, true, work.Done)
	go me.genBookBuildCbz(filepath.Join(outDirPath, bookid+".cbz"), srcfilepaths, lang, bgCol, dirRtl, loRes, work.Done)
	go me.genBookBuildPdf(filepath.Join(outDirPath, bookid+".pdf"), srcfilepaths, lang, bgCol, dirRtl, loRes, work.Done)
	work.Wait()
}

func (*BookBuild) genBookBuildPdf(outFilePath string, srcFilePaths []string, lang string, bgCol bool, dirRtl bool, loRes bool, onDone func()) {
	defer onDone()
	cmdArgs := append(make([]string, 0, 3+len(srcFilePaths)),
		"--pillow-limit-break", "--nodate")
	cmdArgs = append(cmdArgs, srcFilePaths...)
	osExec(true, "img2pdf", append(cmdArgs, "-o", outFilePath)...)
}

func (*BookBuild) genBookBuildCbz(outFilePath string, srcFilePaths []string, lang string, bgCol bool, dirRtl bool, loRes bool, onDone func()) {
	defer onDone()
	outfile, err := os.Create(outFilePath)
	if err != nil {
		panic(err)
	}
	defer outfile.Close()

	zw, numdigits := zip.NewWriter(outfile), len(strconv.Itoa(len(srcFilePaths)))
	for i, srcfilepath := range srcFilePaths {
		if fw, err := zw.Create(itoa0(i+1, numdigits) + filepath.Ext(srcfilepath)); err != nil {
			panic(err)
		} else {
			io.Copy(fw, bytes.NewReader(fileRead(srcfilepath)))
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}

	_ = outfile.Sync()
}
