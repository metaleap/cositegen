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

type DualSize struct {
	MmWidth  int
	MmHeight int
	PxWidth  int
	PxHeight int
}

type BookConfig struct {
	PageSize         DualSize
	CoverSize        DualSize
	PxLoResWidth     int
	MinPageCount     int
	BuildRtlVersions bool
	DecosFromSeries  string
}

type Book struct {
	Config   string
	Name     string
	Title    map[string]string
	Chapters []struct {
		FromSeries                     []string
		ExcludeBySeriesAndChapterNames map[string][]string
		ExcludeBySheetName             []string
		OnlySheetsNamed                []string
		RewriteToMonths                bool
	}
	CssTitle string
	CssToc   string
	CssPgNr  string

	config         *BookConfig
	genPrepDirPath string
}

func (me *Book) id(lang string, bgCol bool, dirRtl bool, loRes bool) string {
	return me.Name + strIf(bgCol, "_col_", "_bw_") + lang + strIf(loRes, "_lo_", "_hi_") +
		strIf(dirRtl, App.Proj.DirModes.Rtl.Name, App.Proj.DirModes.Ltr.Name)
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
					for _, exclname := range chapspec.ExcludeBySeriesAndChapterNames[seriesname] {
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
				for _, exclname := range chapspec.ExcludeBySheetName {
					if excluded = (exclname == sheet.name); excluded {
						break
					}
				}
				if len(chapspec.OnlySheetsNamed) != 0 && !excluded {
					excluded = true
					for _, inclname := range chapspec.OnlySheetsNamed {
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

func (me *Series) genBookPrep(sg *siteGen) {
	book := me.Book
	book.genPrepDirPath = "/dev/shm/" + strconv.FormatInt(time.Now().UnixNano(), 36)
	mkDir(book.genPrepDirPath)
	var sheetsvgfilepaths, pagesvgfilepaths []string
	for lidx, lang := range App.Proj.Langs {
		if lidx > 0 && os.Getenv("BOOKMIN") != "" {
			break
		}
		pgnrs := map[*Chapter]int{}
		for _, bgcol := range []bool{false, true} {
			if bgcol && os.Getenv("BOOKMIN") != "" {
				break
			}
			for _, dirrtl := range []bool{false, true} {
				if dirrtl && (os.Getenv("BOOKMIN") != "" || !book.config.BuildRtlVersions) {
					break
				}
				pgnr := 6
				for _, chap := range me.Chapters {
					pgnrs[chap] = pgnr
					for _, sheet := range chap.sheets {
						sv := sheet.versions[0]
						if skip := (lang != App.Proj.Langs[0] && App.Proj.percentTranslated(lang, me, chap, sv, -1) < 50); skip ||
							(bgcol && !sv.data.hasBgCol) {
							if !skip {
								pgnr++
							}
							continue
						}
						svgfilename := sheet.name + strIf(dirrtl, "_rtl_", "_ltr_") + lang + strIf(bgcol, "_col", "_bw") + ".svg"
						svgfilepath := filepath.Join(book.genPrepDirPath, svgfilename)
						sheetsvgfilepaths = append(sheetsvgfilepaths, svgfilepath)
						me.genBookSheetSvg(sv, svgfilepath, dirrtl, lang, bgcol)
						pagesvgfilename := "p" + itoa0(pgnr, 3) + strIf(dirrtl, "_rtl_", "_ltr_") + lang + strIf(bgcol, "_col", "_bw") + ".svg"
						pagesvgfilepath := filepath.Join(book.genPrepDirPath, pagesvgfilename)
						pagesvgfilepaths = append(pagesvgfilepaths, pagesvgfilepath)
						me.genBookSheetPageSvg(pagesvgfilepath, svgfilepath+".png", [2]int{sv.data.PanelsTree.Rect.Dx(), sv.data.PanelsTree.Rect.Dy()}, pgnr)
						pgnr++
					}
				}
			}
		}
		svgfilepath := filepath.Join(book.genPrepDirPath, "p003_"+lang+".svg")
		pagesvgfilepaths = append(pagesvgfilepaths, svgfilepath)
		me.genBookTiTocPageSvg(svgfilepath, lang, pgnrs)
	}
	pagesvgfilepaths = append(pagesvgfilepaths, me.genBookDirtPageSvgs()...)
	me.genBookTitleTocFacesPng(filepath.Join(book.genPrepDirPath, "faces.png"), &book.config.PageSize, 170, nil)

	mkDir(".ccache/.svgpng")
	for i, svgfilepath := range sheetsvgfilepaths {
		printLn(time.Now().Format("15:04:05"), "shsvg", i, "/", len(sheetsvgfilepaths))
		imgSvgToPng(svgfilepath, svgfilepath+".png", nil, 0, 1200, nil)
	}
	repl := strings.NewReplacer("./", strings.TrimSuffix(book.genPrepDirPath, "/")+"/")
	for i, svgfilepath := range pagesvgfilepaths {
		printLn(time.Now().Format("15:04:05"), "pgsvg", i, "/", len(pagesvgfilepaths))
		var work sync.WaitGroup
		work.Add(1)
		go imgSvgToPng(svgfilepath, svgfilepath+".png", repl, 0, 1200, work.Done)
		if lrw := book.config.PxLoResWidth; lrw > 0 {
			work.Add(1)
			go imgSvgToPng(svgfilepath, svgfilepath+"."+itoa(lrw)+".png", repl, lrw, 0, work.Done)
		}
		work.Wait()
	}
}

func (me *Series) genBookSheetPageSvg(outFilePath string, sheetImgFilePath string, sheetImgSize [2]int, pgNr int) {
	w, h, mm1 := me.Book.config.PageSize.PxWidth, me.Book.config.PageSize.PxHeight, me.Book.config.PageSize.PxHeight/me.Book.config.PageSize.MmHeight
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
		xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
		width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">
		<style type="text/css">
			@font-face { ` + strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) + ` }
			text { ` + strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + "; " + me.Book.CssPgNr + ` }
		</style>`

	mmleft, mmwidth, pgleft := 4, me.Book.config.PageSize.MmWidth-18, 6
	if (pgNr % 2) != 0 {
		mmleft, pgleft = 14, me.Book.config.PageSize.MmWidth-12
	}
	mmheight := int(float64(mmwidth) / (float64(sheetImgSize[0]) / float64(sheetImgSize[1])))
	if mmheight > me.Book.config.PageSize.MmHeight {
		panic(sheetImgFilePath + ": width=" + itoa(mmwidth) + "mm height=" + itoa(mmheight) + "mm")
	}
	mmtop := (me.Book.config.PageSize.MmHeight - mmheight) / 2

	svg += `<image dx="0" dy="0" x="` + itoa(mm1*mmleft) + `" y="` + itoa(mm1*mmtop) + `"
		width="` + itoa(mm1*mmwidth) + `" height="` + itoa(mm1*mmheight) + `"
		xlink:href="./` + filepath.Base(sheetImgFilePath) + `" />`

	svg += `<text dx="0" dy="0" x="` + itoa(pgleft*mm1) + `" y="` + itoa(me.Book.config.PageSize.PxHeight-mm1) + `">` + itoa0(pgNr, 3) + `</text>`

	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *Series) genBookTiTocPageSvg(outFilePath string, lang string, pgNrs map[*Chapter]int) {
	w, h := me.Book.config.PageSize.PxWidth, me.Book.config.PageSize.PxHeight
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
	xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
	width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">`

	svg += `
			<style type="text/css">
			@font-face { ` +
		strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) + ` }
			.title, .toc, g > svg > svg > text, g > svg > svg > text > tspan { ` +
		strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + ` }
			.title { ` + me.Book.CssTitle + ` }
			.toc { ` + me.Book.CssToc + ` }
			</style>
			<image x="0" y="0" width="100%" height="100%" xlink:href="` + filepath.Join(me.Book.genPrepDirPath, "faces.png") + `" />`

	textx, chapcount, pgnrlast := itoa(h/9), 0, 0
	for _, chap := range me.Chapters {
		pgnr := pgNrs[chap]
		if pgnr == pgnrlast {
			continue
		}
		pgnrlast, chapcount = pgnr, chapcount+1
	}

	htoc, cc := 62.0/float64(chapcount), 0
	svg += `<text class="title" x="` + textx + `px" y="30%" dx="0" dy="0">` +
		htmlEscdToXmlEsc(hEsc(locStr(me.Book.Title, lang))) + `</text>`

	pgnrlast = 0
	for _, chap := range me.Chapters {
		pgnr, texty := pgNrs[chap], int(33.0+(float64(cc)+1.0)*htoc)-5
		if pgnr == pgnrlast {
			continue
		}
		svg += `<text class="toc" x="` + textx + `px" y="` + itoa(texty) + `%" dx="0" dy="0">` +
			htmlEscdToXmlEsc(hEsc(locStr(chap.Title, lang)+"············"+App.Proj.textStr(lang, "BookTocPagePrefix")+strIf(pgnr < 10, "0", "")+itoa(pgnr))) + `</text>`
		pgnrlast, cc = pgnr, cc+1
	}

	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *Series) genBookDirtPageSvgs() (outFilePaths []string) {
	var svs []*SheetVer
	rand.Seed(time.Now().UnixNano())
	chaps := me.Chapters
	if forceFrom := App.Proj.seriesByName(me.Book.config.DecosFromSeries); forceFrom != nil {
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

	perpage := float64(len(svs)) / 7.0
	perrowcol := int(math.Ceil(math.Sqrt(perpage)))

	var isv int
	for i := 0; i < 7; i++ {
		w, h := me.Book.config.PageSize.PxWidth, me.Book.config.PageSize.PxHeight
		cw, ch := w/perrowcol, h/perrowcol
		svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
			xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
			width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">`

		for col := 0; col < perrowcol; col++ {
			for row := 0; row < perrowcol; row++ {
				sv, cx, cy := svs[isv], col*cw, row*ch
				isv = (isv + 1) % len(svs)
				scanfilepath, err := filepath.Abs(sv.data.bwFilePath)
				if err != nil {
					panic(err)
				}
				svg += `<image opacity="0.33" x="` + itoa(cx) + `" y="` + itoa(cy) + `" width="` + itoa(cw) + `" height="` + itoa(ch) + `"
					xlink:href="` + scanfilepath + `" />`
			}
		}

		svg += "</svg>"
		outfilepath := filepath.Join(me.Book.genPrepDirPath, "dp"+itoa(i)+".svg")
		outFilePaths = append(outFilePaths, outfilepath)
		fileWrite(outfilepath, []byte(svg))
	}
	return
}

func (me *Series) genBookTitleTocFacesPng(outFilePath string, size *DualSize, inkColor uint8, onDone func()) {
	if onDone != nil {
		defer onDone()
	}

	var svs []*SheetVer
	rand.Seed(time.Now().UnixNano())
	chaps := me.Chapters
	if forceFrom := App.Proj.seriesByName(me.Book.config.DecosFromSeries); forceFrom != nil {
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

	numcols, numrows := 0, 0
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
		ratio := float64(size.PxWidth) / float64(size.PxHeight)
		sort.Slice(grids, func(i int, j int) bool {
			w1, h1 := grids[i], n/grids[i]
			w2, h2 := grids[j], n/grids[j]
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

	cellw, cellh := size.PxWidth/numcols, size.PxHeight/numrows
	img := image.NewGray(image.Rect(0, 0, size.PxWidth, size.PxHeight))
	imgFill(img, image.Rect(0, 0, size.PxWidth, size.PxHeight), color.Gray{255})

	var fidx int
	for fimg, frect := range faces {
		icol, irow, pad := fidx%numcols, fidx/numcols, size.PxHeight/50
		cx, cy, fw, fh := cellw*icol, cellh*irow, frect.Dx(), frect.Dy()
		sf := math.Min(float64(cellw-pad)/float64(fw), float64(cellh-pad)/float64(fh)) //scale factor
		dw, dh := int(float64(fw)*sf), int(float64(fh)*sf)
		dx, dy := cx+((cellw-dw)/2), cy+((cellh-dh)/2)
		drect := image.Rect(dx, dy, dx+dw, dy+dh)
		ImgScaler.Scale(img, drect, fimg, frect, draw.Over, nil)
		fidx++
	}

	var buf bytes.Buffer
	if err := PngEncoder.Encode(&buf, img); err != nil {
		panic(err)
	}
	fileWrite(outFilePath, buf.Bytes())
}

func (me *Series) genBookSheetSvg(sv *SheetVer, outFilePath string, dirRtl bool, lang string, bgCol bool) {
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

		px, py, pw, ph := p.Rect.Min.X, p.Rect.Min.Y, p.Rect.Dx(), p.Rect.Dy()
		tx, gid := px, "pnl"+itoa(pidx)
		if dirRtl {
			tx = w - pw - px
		}
		svg += `<g id="` + gid + `" transform="translate(` + itoa(tx) + ` ` + itoa(py) + `)">`
		if bgCol {
			panelbgpngsrcfilepath, err := filepath.Abs(filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".png"))
			if err != nil {
				panic(err)
			}
			svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
				xlink:href="` + panelbgpngsrcfilepath + `" />`
		} else {
			svg += `<rect x="0" y="0" stroke="#000000" stroke-width="0" fill="#ffffff"
				width="` + itoa(pw) + `" height="` + itoa(ph) + `"></rect>`
		}
		svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
				xlink:href="` + panelpngsrcfilepath + `" />`
		svg += sv.genTextSvgForPanel(pidx, p, lang, false)
		svg += "\n</g>\n\n"
		pidx++
	})
	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *Series) genBookBuild(outDirPath string, lang string, bgCol bool, dirRtl bool, loRes bool, onDone func()) {
	defer onDone()
	book, pgnr, idp, srcfilepaths := me.Book, 1, 0, make([]string, 0, me.numSheets())

	for ; pgnr <= 5; pgnr++ {
		srcfilepath := filepath.Join(book.genPrepDirPath, "dp"+itoa(idp)+".svg"+strIf(loRes, "."+itoa(book.config.PxLoResWidth), "")+".png")
		if pgnr == 3 {
			srcfilepath = filepath.Join(book.genPrepDirPath, "p003_"+lang+".svg"+strIf(loRes, "."+itoa(book.config.PxLoResWidth), "")+".png")
		} else {
			idp = (idp + 1) % 7
		}
		srcfilepaths = append(srcfilepaths, srcfilepath)
	}
	for _, chap := range me.Chapters {
		for _, sheet := range chap.sheets {
			sv := sheet.versions[0]
			if lang != App.Proj.Langs[0] && App.Proj.percentTranslated(lang, me, chap, sv, -1) < 50 {
				continue
			}
			srcfilepaths = append(srcfilepaths, filepath.Join(book.genPrepDirPath,
				"p"+itoa0(pgnr, 3)+strIf(dirRtl, "_rtl_", "_ltr_")+lang+strIf(bgCol && sv.data.hasBgCol, "_col", "_bw")+".svg"+strIf(loRes, "."+itoa(book.config.PxLoResWidth), "")+".png"))
			pgnr++
		}
	}
	for numtrailingempties := 0; !(numtrailingempties >= 4 && (len(srcfilepaths)%4) == 0 && len(srcfilepaths) >= book.config.MinPageCount); numtrailingempties++ {
		srcfilepaths = append(srcfilepaths, filepath.Join(book.genPrepDirPath, "dp"+itoa(idp)+".svg"+strIf(loRes, "."+itoa(book.config.PxLoResWidth), "")+".png"))
		idp = (idp + 1) % 7
	}

	var work sync.WaitGroup
	bookid := me.Book.id(lang, bgCol, dirRtl, loRes)

	work.Add(1)
	go me.genBookBuildCbz(filepath.Join(outDirPath, bookid+".cbz"), srcfilepaths, lang, bgCol, dirRtl, loRes, work.Done)

	work.Add(1)
	go me.genBookBuildPdf(filepath.Join(outDirPath, bookid+".pdf"), srcfilepaths, lang, bgCol, dirRtl, loRes, work.Done)

	work.Wait()
}

func (*Series) genBookBuildPdf(outFilePath string, srcFilePaths []string, lang string, bgCol bool, dirRtl bool, loRes bool, onDone func()) {
	defer onDone()
	cmdArgs := append(make([]string, 0, 3+len(srcFilePaths)),
		"--pillow-limit-break", "--nodate")
	cmdArgs = append(cmdArgs, srcFilePaths...)
	osExec(true, "img2pdf", append(cmdArgs, "-o", outFilePath)...)
}

func (*Series) genBookBuildCbz(outFilePath string, srcFilePaths []string, lang string, bgCol bool, dirRtl bool, loRes bool, onDone func()) {
	defer onDone()
	outfile, err := os.Create(outFilePath)
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	zw := zip.NewWriter(outfile)

	for i, srcfilepath := range srcFilePaths {
		filename := filepath.Base(srcfilepath)
		if strings.HasPrefix(filename, "dp") && strings.Contains(filename, ".svg.") && strings.HasSuffix(filename, ".png") {
			filename = "p" + itoa0(i+1, 3) + ".png"
		}
		var data = fileRead(srcfilepath)
		if fw, err := zw.Create(filename); err != nil {
			panic(err)
		} else {
			io.Copy(fw, bytes.NewReader(data))
		}
	}

	if err := zw.Close(); err != nil {
		panic(err)
	}
	_ = outfile.Sync()
}
