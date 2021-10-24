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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type BookConfig struct {
	KeepUntranslated bool
	PxWidth          int
	PxHeight         int
	MmWidth          int
	MmHeight         int
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
	CssTitle string
	CssToc   string
	CssPgNr  string

	config  *BookConfig
	genPrep struct {
		files      map[string]map[string]string
		imgDirPath string
	}
}

func (me *Book) id(lang string, bgCol bool, dirRtl bool) string {
	return me.Name + "_" + lang + strIf(bgCol, "_col_", "_bw_") +
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

	book.genPrep.imgDirPath = "/dev/shm/" + strconv.FormatInt(time.Now().UnixNano(), 36)
	mkDir(book.genPrep.imgDirPath)
	var sheetsvgfilepaths, pagesvgfilepaths []string
	for _, lang := range App.Proj.Langs {
		for _, dirRtl := range []bool{false, true} {
			for _, bgCol := range []bool{false, true} { // keep `false` first, `true` second
				pgnr := 6
				for _, chap := range me.Chapters {
					for _, sheet := range chap.sheets {
						sv := sheet.versions[0]
						if bgCol && !sv.data.hasBgCol {
							continue
						}
						svgfilename := sheet.name + strIf(dirRtl, "_rtl_", "_ltr_") + lang + strIf(bgCol, "_col", "_bw") + ".svg"
						svgfilepath := filepath.Join(book.genPrep.imgDirPath, svgfilename)
						sheetsvgfilepaths = append(sheetsvgfilepaths, svgfilepath)
						me.genBookSheetSvg(sv, svgfilepath, dirRtl, lang, bgCol)
						pagesvgfilename := "p" + itoa0(pgnr, 3) + strIf(dirRtl, "_rtl_", "_ltr_") + lang + strIf(bgCol, "_col", "_bw") + ".svg"
						pagesvgfilepath := filepath.Join(book.genPrep.imgDirPath, pagesvgfilename)
						pagesvgfilepaths = append(pagesvgfilepaths, pagesvgfilepath)
						me.genBookSheetPageSvg(pagesvgfilepath, svgfilepath+".png", [2]int{sv.data.PanelsTree.Rect.Dx(), sv.data.PanelsTree.Rect.Dy()}, pgnr)
						pgnr++
					}
				}
			}
		}
		svgfilepath := filepath.Join(book.genPrep.imgDirPath, "p"+itoa0(3, 3)+"_"+lang+".svg")
		pagesvgfilepaths = append(pagesvgfilepaths, svgfilepath)
		me.genBookTiTocPageSvg(svgfilepath, lang)
	}
	me.genBookTitleTocFacesPng(filepath.Join(book.genPrep.imgDirPath, "faces.png"))
	{
		imgwhite := image.NewGray(image.Rect(0, 0, book.config.PxWidth, book.config.PxHeight))
		imgFill(imgwhite, imgwhite.Bounds(), color.Gray{255})
		var buf bytes.Buffer
		PngEncoder.Encode(&buf, imgwhite)
		fileWrite(filepath.Join(book.genPrep.imgDirPath, "p000.png"), buf.Bytes())
	}
	for _, svgfilepath := range sheetsvgfilepaths {
		imgSvgToPng(svgfilepath, svgfilepath+".png")
	}
	for _, svgfilepath := range pagesvgfilepaths {
		imgSvgToPng(svgfilepath, svgfilepath+".png")
	}
}

func (me *Series) genBookSheetPageSvg(outFilePath string, sheetImgFilePath string, sheetImgSize [2]int, pgNr int) {
	w, h, mm1 := me.Book.config.PxWidth, me.Book.config.PxHeight, me.Book.config.PxHeight/me.Book.config.MmHeight
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
		xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
		width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">
		<style type="text/css">
			@font-face { ` + strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) + ` }
			text { ` + strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + "; " + me.Book.CssPgNr + ` }
		</style>`

	mmleft, mmwidth, pgleft := 5, me.Book.config.MmWidth-20, 6
	if (pgNr % 2) != 0 {
		mmleft, pgleft = 15, me.Book.config.MmWidth-16
	}
	mmheight := int(float64(mmwidth) / (float64(sheetImgSize[0]) / float64(sheetImgSize[1])))
	if mmheight > me.Book.config.MmHeight {
		panic(sheetImgFilePath + ": width=" + itoa(mmwidth) + "mm height=" + itoa(mmheight) + "mm")
	}
	mmtop := (me.Book.config.MmHeight - mmheight) / 2

	svg += `<image dx="0" dy="0" x="` + itoa(mm1*mmleft) + `" y="` + itoa(mm1*mmtop) + `"
		width="` + itoa(mm1*mmwidth) + `" height="` + itoa(mm1*mmheight) + `"
		xlink:href="` + sheetImgFilePath + `" />`

	svg += `<text dx="0" dy="0" x="` + itoa(pgleft*mm1) + `" y="` + itoa(me.Book.config.PxHeight-mm1) + `">` + itoa0(pgNr, 3) + `</text>`

	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *Series) genBookTiTocPageSvg(outFilePath string, lang string) {
	w, h := me.Book.config.PxWidth, me.Book.config.PxHeight
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
			<image x="0" y="0" width="100%" height="100%"
			xlink:href="` + filepath.Join(me.Book.genPrep.imgDirPath, "faces.png") + `" />`

	textx := itoa(h / 9)
	htoc := 62.0 / float64(len(me.Chapters))
	svg += `<text class="title" x="` + textx + `px" y="33%" dx="0" dy="0">` +
		htmlEscdToXmlEsc(hEsc(locStr(me.Book.Title, lang))) + `</text>`

	pgnr := 6
	for i, chap := range me.Chapters {
		texty := int(33.0+(float64(i)+1.0)*htoc) - 5
		svg += `<text class="toc" x="` + textx + `px" y="` + itoa(texty) + `%" dx="0" dy="0">` +
			htmlEscdToXmlEsc(hEsc(locStr(chap.Title, lang)+"············"+App.Proj.textStr(lang, "BookTocPagePrefix")+strIf(pgnr < 10, "0", "")+itoa(pgnr))) + `</text>`
		pgnr += len(chap.sheets)
	}

	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *Series) genBookTitleTocFacesPng(outFilePath string) {
	faces := map[*image.Gray]image.Rectangle{}
	var work sync.WaitGroup
	var lock sync.Mutex
	for _, chap := range me.Chapters {
		for _, sheet := range chap.sheets {
			if sv := sheet.versions[0]; sv.hasFaceAreas() {
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
							fimg := image.NewGray(area.Rect)
							for x := fimg.Bounds().Min.X; x < fimg.Bounds().Max.X; x++ {
								for y := fimg.Bounds().Min.Y; y < fimg.Bounds().Max.Y; y++ {
									gray := subimg.GrayAt(x, y)
									if gray.Y == 0 {
										gray.Y = 177
									} else if gray.Y != 255 {
										panic(gray.Y)
									}
									fimg.SetGray(x, y, gray)
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
		}
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
		ratio := float64(me.Book.config.PxWidth) / float64(me.Book.config.PxHeight)
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

	cellw, cellh := me.Book.config.PxWidth/numcols, me.Book.config.PxHeight/numrows
	img := image.NewGray(image.Rect(0, 0, me.Book.config.PxWidth, me.Book.config.PxHeight))
	imgFill(img, image.Rect(0, 0, me.Book.config.PxWidth, me.Book.config.PxHeight), color.Gray{255})

	var fidx int
	for fimg, frect := range faces {
		icol, irow, pad := fidx%numcols, fidx/numcols, me.Book.config.PxHeight/50
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

func (me *Series) genBookBuild(outDirPath string, lang string, bgCol bool, dirRtl bool, onDone func()) {
	defer onDone()
	bookid := me.Book.id(lang, bgCol, dirRtl)
	var work sync.WaitGroup
	work.Add(2)
	go me.genBookBuildCbz(filepath.Join(outDirPath, bookid+".cbz"), lang, bgCol, dirRtl, work.Done)
	go me.genBookBuildEpub(bookid, filepath.Join(outDirPath, bookid+".epub"), lang, bgCol, dirRtl, work.Done)
	work.Wait()
}

func (me *Series) genBookBuildCbz(outFilePath string, lang string, bgCol bool, dirRtl bool, onDone func()) {
	defer onDone()
}

func (me *Series) genBookBuildEpub(bookId string, outFilePath string, lang string, bgCol bool, dirRtl bool, onDone func()) {
	defer onDone()
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
