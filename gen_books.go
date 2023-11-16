package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	bookScreenWidth        = 3744
	bookScreenBorder       = 44
	bookScreenLoResDiv     = 4
	bookPrintBorderMmBig   = 15
	bookPrintBorderMmLil   = 7
	bookPrintBorderMmShift = 3
	bookPanelsHPadding     = 188
)

var (
	bookScreenPgBgCol = [3]uint8{0xe7, 0xe2, 0xdb}
	bookGenCssRepl    = strings.NewReplacer("./", strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/")
)

type BookGen struct {
	Sheets         []*SheetVer
	Phrase         string
	ShmDirPath     string
	OutDirPath     string
	MaxSheetWidth  int
	MaxSheetHeight int

	year           int
	facesFilePaths []string
	perRow         struct {
		vertText  string
		firstOnly bool
	}
}

func makeBook(flags map[string]bool) {
	phrase := strings.Join(os.Args[2:], "-")
	gen := BookGen{
		Phrase:     phrase,
		ShmDirPath: "/dev/shm/" + phrase,
		OutDirPath: ".books/" + phrase,
	}
	rmDir(gen.ShmDirPath)
	rmDir(gen.OutDirPath)
	mkDir(gen.ShmDirPath)
	mkDir(gen.OutDirPath)

	for k := range flags {
		if y := atoi(k, 0, 9999); y > 2020 && y < 3131 {
			gen.year = y
		}
	}
	for _, series := range App.Proj.Series {
		for _, chap := range series.Chapters {
			if y := itoa(gen.year); (chap.Name == y || chap.UrlName == y) && len(flags) > 1 {
				continue
			}
			if (flags[chap.Name] || flags[chap.UrlName] ||
				flags[series.Name] || flags[series.UrlName]) &&
				(gen.year == 0 || chap.scanYearHas(gen.year, true)) {
				for _, sheet := range chap.sheets {
					sv := sheet.versions[0]
					gen.Sheets = append(gen.Sheets, sv)

					rect := sv.data.pxBounds()
					if w := rect.Dx(); w > gen.MaxSheetWidth {
						gen.MaxSheetWidth = w
					}
					if h := rect.Dy(); h > gen.MaxSheetHeight {
						gen.MaxSheetHeight = h
					}
				}
			}
		}
	}
	sort.SliceStable(gen.Sheets, func(i int, j int) bool {
		return gen.Sheets[i].DtStr() < gen.Sheets[j].DtStr()
	})
	for i := 0; i < len(gen.Sheets); i++ { // eg REPL_41NCSGP=57SCBVF
		if repl := os.Getenv("REPL_" + gen.Sheets[i].parentSheet.name); repl != "" {
			for j, svr := range gen.Sheets {
				if svr.parentSheet.name == repl {
					gen.Sheets = append(gen.Sheets[:j], gen.Sheets[j+1:]...)
					gen.Sheets[i] = svr
					i = -1
					break
				}
			}
		}
	}
	if len(gen.Sheets) == 0 {
		panic("no scans found for: " + phrase)
	}

	var coverdone bool
	for _, dirrtl := range []bool{false, true /*KEEP this order of bools*/} {
		for _, lang := range App.Proj.Langs {
			if lang != os.Getenv("LANG") {
				continue
			}
			gen.genSheetSvgsAndPngs(dirrtl, lang)
			if os.Getenv("NOSCREEN") == "" {
				gen.genScreenVersion(dirrtl, lang)
			}
			if os.Getenv("NOPRINT") == "" {
				numpages := atoi(os.Getenv("ONLYCOV"), 0, 999)
				if numpages == 0 {
					numpages = gen.genPrintVersion(dirrtl, lang)
				}
				if title := os.Getenv("TITLE"); title != "" && !coverdone {
					coverdone = true
					gen.genPrintCover(title, numpages)
				}
			}
			if os.Getenv("NOLANG") != "" {
				break
			}
		}
		if os.Getenv("NORTL") != "" || os.Getenv("NODIR") != "" {
			break
		}
	}
}

func (me *BookGen) genSheetSvgsAndPngs(dirRtl bool, lang string) {
	lores := (os.Getenv("LORES") != "")
	for i, sv := range me.Sheets {
		if os.Getenv("NOPRINT") == "" {
			sheetsvgfilepath := me.sheetSvgPath(i, dirRtl, lang, true)
			me.genSheetSvg(sv, sheetsvgfilepath, dirRtl, lang, true, "white")
		}
		if os.Getenv("NOSCREEN") == "" {
			sheetsvgfilepath := me.sheetSvgPath(i, dirRtl, lang, false)
			me.genSheetSvg(sv, sheetsvgfilepath, dirRtl, lang, false, "#"+itoh(bookScreenPgBgCol[0])+itoh(bookScreenPgBgCol[1])+itoh(bookScreenPgBgCol[2]))
			sheetpngfilepath := sheetsvgfilepath + ".sh.png"
			printLn(sheetpngfilepath, "...")
			imgAnyToPng(sheetsvgfilepath, sheetpngfilepath, iIf(!lores, 0, bookScreenWidth/bookScreenLoResDiv), false, sIf(!lores, "sh_", "sh_lq_"))
		}
	}
}

func (me *BookGen) genSheetSvg(sv *SheetVer, outFilePath string, dirRtl bool, lang string, skewForPrint bool, polyBgCol string) {
	rectinner, lores := sv.data.pxBounds(), (os.Getenv("LORES") != "")

	w, h := rectinner.Dx(), rectinner.Dy()
	svgtxt := sv.parentSheet.parentChapter.GenPanelSvgText
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
        xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
        width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">
            <style type="text/css">` + App.Proj.cssFontFaces(bookGenCssRepl) + `
				polygon.pt, polygon.ptb { stroke: black; fill: ` + polyBgCol + `; }
				tspan.sidetxt { font-size: 177px; stroke-width: 22px !important; }
				text.sidetxt { transform: rotate(-90deg); }
				g > svg > svg > text, g > svg > svg > text > tspan, tspan.sidetxt {
					`
	for _, k := range sortedMapKeys(svgtxt.Css[""]) {
		svg += k + ":" + svgtxt.Css[""][k] + ";\n"
	}

	svg += `}`
	if me.year > 0 && me.year <= 2022 {
		svg += `
				g > svg > svg > text > tspan.std { /*_un_bold_*/ }
				g > svg > svg > text > tspan.std tspan.b { font-weight: bold !important; }`
	} else if skewForPrint && sv.DtStr() < "20230301" {
		svg += `g > svg > svg > text > tspan { letter-spacing: -0.006em !important; }`
	}
	svg += `</style>`

	for i := range sv.data.PanelsTree.SubRows {
		if row := &sv.data.PanelsTree.SubRows[i]; len(row.SubCols) > 1 {
			row.setTopLevelRowRecenteredX(sv.data.PanelsTree, w, h)
		}
	}

	pidx, qidx := 0, iIf(lores, 0, App.Proj.maxQualiIdx(false))
	rowmids, ymid := map[int]int{}, -1
	sv.data.PanelsTree.each(func(p *ImgPanel) {
		px, py, pw, ph := p.Rect.Min.X+p.recenteredXOffset, p.Rect.Min.Y-rectinner.Min.Y, p.Rect.Dx(), p.Rect.Dy()
		if py != ymid && (len(rowmids) == 0 || !me.perRow.firstOnly) {
			ymid = py
			rowmids[py+(ph/2)] = px + pw + 128 + 6
		}
		tx, gid := px, "pnl"+itoa(pidx)
		if dirRtl {
			tx = w - pw - px
		}
		svg += `<g id="` + gid + `" clip-path="url(#c` + gid + `)" transform="translate(` + itoa(tx) + ` ` + itoa(py) + `)">`
		svg += `<defs><clipPath id="c` + gid + `"><rect x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"></rect></clipPath></defs>`
		if panelbgpngsrcfilepath := filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".png"); fileStat(panelbgpngsrcfilepath) != nil {
			svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
						xlink:href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(fileRead(panelbgpngsrcfilepath)) + `" />`
		} else {
			svg += `<rect x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
						fill="#ffffff" stroke-width="0" />`
		}
		svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
					xlink:href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(fileRead(filepath.Join(sv.data.PicDirPath(App.Proj.Qualis[qidx].SizeHint), itoa(pidx)+".png"))) + `" />
					`
		if lang != "" {
			svg += sv.genTextSvgForPanel(pidx, p, lang, false, true)
		}
		svg += "\n</g>\n\n"
		pidx++
	})

	if me.perRow.vertText != "" {
		for y, x := range rowmids {
			svg += `<g x="0" y="0" transform="translate(` + itoa(x) + ` ` + itoa(y) + `)"><text class="sidetxt"><tspan dx="0" dy="0" class="sidetxt">` + xEsc(me.perRow.vertText) + `</tspan></text></g>`
		}
	}

	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *BookGen) sheetSvgPath(idx int, dirRtl bool, lang string, forPrint bool) string {
	return me.ShmDirPath + "/" + sIf(dirRtl, "r", "l") + sIf(forPrint, "p", "s") + itoa0pref(idx, 3) + lang + sIf(os.Getenv("LORES") == "", "", "_lq") + ".svg"
}

func (me *BookGen) genScreenVersion(dirRtl bool, lang string) {
	border, pgw, pgh, lores := bookScreenBorder, bookScreenWidth, int(float64(bookScreenWidth)/(float64(me.MaxSheetWidth)/float64(me.MaxSheetHeight))), (os.Getenv("LORES") != "")
	if lores {
		pgw, pgh, border = pgw/bookScreenLoResDiv, pgh/bookScreenLoResDiv, border/bookScreenLoResDiv
	}

	pgfilepaths := []string{}
	if os.Getenv("NOTOC") == "" {
		tocfilepathsvg := me.ShmDirPath + "/0toc." + lang + sIf(!lores, "", "_lq") + ".svg"
		tocfilepathpng := tocfilepathsvg + ".png"
		if fileStat(tocfilepathpng) == nil {
			svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
				width="` + itoa(pgw) + `" height="` + itoa(pgh) + `" style="background-color: #ffffff">
				<style type="text/css">
					text.toc tspan {
						font-family: "Shark Heavy ABC";
						font-size: ` + sIf(!lores, "8.88", "2.22") + `em;
						font-weight: normal;
						paint-order: stroke;
						stroke: #ffffff;
						stroke-width: ` + sIf(!lores, "4", "1") + `mm;
						white-space: pre;
					}
					text.toctitle tspan {
						font-family: "Shark Heavy ABC";
						font-weight: normal;
						font-size: ` + sIf(!lores, "20", "5") + `em;
						paint-order: stroke;
						stroke: #000000;
						stroke-width: ` + sIf(!lores, "8", "2") + `mm;
						fill: #ffffff;
					}
					text.tocsub tspan {
						font-family: "Gloria Hallelujah";
						font-size: ` + sIf(!lores, "4", "1") + `em;
						font-weight: bold;
						stroke: #ffffff;
						stroke-width: 0.044em;
					}
					image {
						opacity: 0.22;
					}
				</style>
			`
			svg += me.tocSvg(lang, pgw, pgh) + "</svg>"
			fileWrite(tocfilepathsvg, []byte(svg))
			printLn(tocfilepathpng, "...")
			if writtenfilepath := imgAnyToPng(tocfilepathsvg, tocfilepathpng, 0, true, sIf(!lores, "toc_", "toc_lq_")); os.Getenv("NOZOP") == "" {
				pngOptFireAndForget(writtenfilepath)
			}
		}
		pgfilepaths = append(pgfilepaths, tocfilepathpng)
	}

	for i := range me.Sheets {
		shfilepath := me.sheetSvgPath(i, dirRtl, lang, false) + ".sh.png"
		outfilepath := me.sheetSvgPath(i, dirRtl, lang, false) + ".pg.png"
		printLn(outfilepath, "...")
		tmpfilepath := ".ccache/.pngtmp/pgsh_" + itoa(border) + "_" + itoa(pgw) + "_" + contentHashStr(fileRead(shfilepath)) + ".png"
		if fileStat(tmpfilepath) == nil {
			imgpg := image.NewNRGBA(image.Rect(0, 0, pgw, pgh))
			imgFill(imgpg, imgpg.Bounds(), color.NRGBA{R: bookScreenPgBgCol[0], G: bookScreenPgBgCol[1], B: bookScreenPgBgCol[2], A: 255})
			imgsh, _, err := image.Decode(bytes.NewReader(fileRead(shfilepath)))
			if err != nil {
				panic(err)
			}
			shw := pgw - (2 * border)
			shh := int(float64(shw) / (float64(imgsh.Bounds().Dx()) / float64(imgsh.Bounds().Dy())))
			if pghb := pgh - (2 * border); shh > pghb {
				f := float64(shh) / float64(pghb)
				shw, shh = int(float64(shw)/f), pghb
			}
			shx, shy := (pgw-shw)/2, (pgh-shh)/2
			ImgScaler.Scale(imgpg, image.Rect(shx, shy, shx+shw, shy+shh), imgsh, imgsh.Bounds(), draw.Over, nil)
			fileWrite(tmpfilepath, pngEncode(imgpg))
			if os.Getenv("NOZOP") == "" && !lores {
				pngOptFireAndForget(tmpfilepath)
			}
		}
		fileLink(tmpfilepath, outfilepath)
		pgfilepaths = append(pgfilepaths, outfilepath)
	}

	if altsvgfilepath := "stuff/" + me.Phrase + "/collage.svg"; fileStat(altsvgfilepath) != nil {
		outfilepath := me.sheetSvgPath(len(pgfilepaths), true, "", false) + ".png"
		printLn(outfilepath, "...")
		if writtenfilepath := imgAnyToPng(altsvgfilepath, outfilepath, pgw, false, "xtra"); os.Getenv("NOZOP") == "" {
			pngOptFireAndForget(writtenfilepath)
		}
		pgfilepaths = append(pgfilepaths, outfilepath)
	}

	if os.Getenv("NOCBZ") == "" {
		outfilepathcbz := me.OutDirPath + "/" + bookFileName(me.Phrase, "screen", lang, dirRtl, ".cbz")
		printLn(outfilepathcbz, "...")
		outfile, err := os.Create(outfilepathcbz)
		if err != nil {
			panic(err)
		}
		defer outfile.Close()
		zw := zip.NewWriter(outfile)
		for _, srcfilepath := range pgfilepaths {
			if fw, err := zw.Create(filepath.Base(srcfilepath)); err != nil {
				panic(err)
			} else if _, err = io.Copy(fw, bytes.NewReader(fileRead(srcfilepath))); err != nil {
				panic(err)
			}
		}
		if err := zw.Close(); err != nil {
			panic(err)
		} else if err = outfile.Sync(); err != nil {
			panic(err)
		}
	}

	if os.Getenv("NOPDF") == "" {
		outfilepathpdf := me.OutDirPath + "/" + bookFileName(me.Phrase, "screen", lang, dirRtl, ".pdf")
		printLn(outfilepathpdf, "...")
		cmdArgs := []string{"--pillow-limit-break", "--nodate",
			"--pagesize", "A5^T"}
		cmdArgs = append(cmdArgs, pgfilepaths...)
		if s := osExec(false, nil, "img2pdf", append(cmdArgs, "-o", outfilepathpdf)...); strings.Contains(s, "error:") {
			panic(s)
		}
	}
}

func (me *BookGen) genPrintVersion(dirRtl bool, lang string) (numPages int) {
	svgh, pgwmm, pghmm, isoddpage, pgidx, dpnope := 0, 210, 297, false, -1, os.Getenv("NOTOC") != "" && os.Getenv("NOCOV") != ""
	for numPages = iIf(dpnope, 0, 2+iIf(os.Getenv("NOTOC") == "", 4, 2)) + len(me.Sheets)/2; (numPages%4) != 0 && !dpnope; {
		numPages++
	}
	svg := ""
	dpbwidx, dpbwfilepaths := 0, make([]string, 0, len(me.Sheets))
	if !dpnope {
		for _, sv := range me.Sheets {
			dpbwfilepaths = append(dpbwfilepaths, absPath(sv.data.bwSmallFilePath))
		}
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(dpbwfilepaths), func(i int, j int) {
			dpbwfilepaths[i], dpbwfilepaths[j] = dpbwfilepaths[j], dpbwfilepaths[i]
		})
	}
	svgpgstart := func() {
		isoddpage = !isoddpage
		pgidx++
		y := float64(pgidx*pghmm) + (float64(1+pgidx) * 0.125)
		svg += `<svg class="pg" x="0" y="` + ftoa(y, -1) + `mm" width="` + itoa(pgwmm) + `mm" height="` + itoa(pghmm) + `mm">`
		svgh = int(y) + pghmm
	}
	dpadd := func(closeTag bool) {
		if dpnope {
			return
		}
		svgpgstart()
		x, y := -55, -20
		for {
			if x > pgwmm {
				x, y = -(11 + rand.Intn(22)), y+33
			} else {
				x += 46
			}
			if x > pgwmm && y > pghmm {
				break
			}
			if dpbwidx++; dpbwidx == len(dpbwfilepaths) {
				dpbwidx = 0
			}
			even := ((dpbwidx % 2) == 0)
			svg += `<image width="44mm" x="` + itoa(x+iIf(even, 0, 0)) + `mm" y="` + itoa(y) + `mm"
						xlink:href="file://` + dpbwfilepaths[dpbwidx] + `"
						opacity="` + sIf(closeTag, "0.22", "0.11") + `" transform="rotate(` + itoa(iIf(even, 2, -2)) + `)" />`
		}
		if closeTag {
			svg += "</svg>"
		}
	}
	dpadd(true)
	dpadd(true)
	if os.Getenv("NOTOC") == "" {
		dpadd(false)
		svg += me.tocSvg(lang, 0, 0) + "</svg>"
		dpadd(true)
	}
	repl2022 := strings.NewReplacer(
		"/*_un_bold_*/", "font-weight: normal !important;",
	)
	svg2base64 := func(svgfilepath string, inlineHrefs bool) string {
		src := fileRead(svgfilepath)
		s := string(src)
		if me.year <= 2022 {
			for again := true; again; { // keep sync'd the below needles with consts in SheetVer.imgSvgText
				idx1, idx2 := strings.Index(s, "<tspan class='i' font-style='italic'><tspan class='u' text-decoration='underline'>"), strings.Index(s, "<tspan class='u' text-decoration='underline'><tspan class='i' font-style='italic'>")
				if again = false; idx1 >= 0 || idx2 >= 0 {
					if idx1 < 0 || (idx2 >= 0 && idx2 < idx1) {
						idx1 = idx2
					}
					if idx2 = idx1 + strings.Index(s[idx1:], "</tspan></tspan>"); idx2 > idx1 {
						str := s[idx1+82 : idx2]
						s = s[:idx1] + "<tspan class='b' font-weight='bold'>" + str + "</tspan>" + s[idx2+16:]
						again = true
					}
				}
			}
			s = repl2022.Replace(s)
		}
		if s1, s2 := "xlink:href=\"", "xlink_href=\""; inlineHrefs {
			for i1 := strings.Index(s, s1); i1 > 0; i1 = strings.Index(s, s1) {
				i2 := i1 + len(s1) + strings.IndexByte(s[i1+len(s1):], '"')
				href := s[i1+len(s1) : i2]
				if fp := filepath.Join("stuff", me.Phrase, href); fileStat(fp) != nil {
					href = "data:image/" + strings.TrimPrefix(filepath.Ext(href), ".") + ";base64," + base64.StdEncoding.EncodeToString(fileRead(fp))
				}
				s = s[:i1] + s2 + href + s[i2:]
			}
			s = strings.ReplaceAll(s, s2, s1)
		}
		return base64.StdEncoding.EncodeToString([]byte(s))
	}
	for i, l := 0, (len(me.Sheets)/2)+(len(me.Sheets)%2); i < l; i++ {
		svgpgstart()
		sheetsvgfilepath0 := me.sheetSvgPath(i*2, dirRtl, lang, true)
		sheetsvgfilepath1 := me.sheetSvgPath((i*2)+1, dirRtl, lang, true)
		svg += `<text x="50%" y="97%"><tspan>` + itoa(pgidx+1) + `</tspan></text>`
		topborder := bookPrintBorderMmBig
		if me.Sheets[i*2].parentSheet.parentChapter.Name == "half-pagers" {
			topborder = bookPrintBorderMmLil
		}
		x := iIf(isoddpage, bookPrintBorderMmBig+bookPrintBorderMmShift, bookPrintBorderMmLil-bookPrintBorderMmShift)
		w := pgwmm - (bookPrintBorderMmBig + bookPrintBorderMmLil)
		svg += `<image x="` + itoa(x) + `mm" y="` + itoa(topborder) + `mm" width="` + itoa(w) + `mm" xlink:href="data:image/svg+xml;base64,` + svg2base64(sheetsvgfilepath0, false) + `"/>`
		if fileStat(sheetsvgfilepath1) != nil {
			svg += `<image x="` + itoa(x) + `mm" y="` + ftoa(fIf(strings.HasPrefix(me.Sheets[i*2].parentSheet.name, "01FROGF"), 47, 50.2), 1) + `%" width="` + itoa(w) + `mm" xlink:href="data:image/svg+xml;base64,` + svg2base64(sheetsvgfilepath1, false) + `"/>`
		} else if altsvgfilepath := "stuff/" + me.Phrase + "/collage.svg"; fileStat(altsvgfilepath) != nil {
			svg += `<image x="` + itoa(x) + `mm" y="51%" width="` + itoa(w) + `mm" xlink:href="data:image/svg+xml;base64,` + svg2base64(altsvgfilepath, true) + `"/>`
		}
		svg += "</svg>"
	}
	dpadd(true)
	for (1 + pgidx) < numPages {
		dpadd(true)
	}

	svgtxt := App.Proj.Sheets.Panel.SvgText[""]
	svgfull := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
				width="` + itoa(pgwmm) + `mm" height="` + itoa(svgh) + `mm">
				<style type="text/css">
				` + App.Proj.cssFontFaces(bookGenCssRepl) + `
					@page { margin: 0; padding: 0; line-height: unset; size: ` + itoa(pgwmm) + `mm ` + itoa(pghmm) + `mm; }
					* { margin: 0; padding: 0; line-height: unset; }
					svg.pg { page-break-after: always; break-after: always;}
					image { transform-origin: center; transform-box: fill-box; }
					text, text > tspan {
`
	for _, k := range sortedMapKeys(svgtxt.Css[""]) {
		svgfull += k + ":" + svgtxt.Css[""][k] + ";\n"
	}

	svgfull += ` 	}
					text.toc tspan {
						font-family: "Shark Heavy ABC";
						font-size: 1.11cm;
						font-weight: normal;
						paint-order: stroke;
						stroke: #ffffff;
						stroke-width: 1mm;
						white-space: pre;
					}
					text.toctitle tspan {
						font-family: "Shark Heavy ABC";
						font-weight: normal;
						font-size: 2.88cm;
						paint-order: stroke;
						stroke: #000000;
						stroke-width: 2.88mm;
						fill: #ffffff;
					}
					text.tocsub tspan {
						font-family: "Gloria Hallelujah";
						font-size: 1em;
						font-weight: normal !important;
						stroke-width: 0.123em;
						stroke: #ffffff;
					}
				</style>` + svg + "</svg>"

	outfilepathsvg := me.ShmDirPath + "/print_" + lang + sIf(dirRtl, "_rtl", "_ltr") + ".svg"
	fileWrite(outfilepathsvg, []byte(svgfull))
	if os.Getenv("NOPDF") == "" {
		me.printSvgToPdf(outfilepathsvg, me.OutDirPath+"/"+bookFileName(me.Phrase, "print", lang, dirRtl, ".pdf"))
	}
	return
}

func (me *BookGen) tocSvg(lang string, pgW int, pgH int) (s string) {
	var tocs []int
	for i, sv := range me.Sheets {
		if len(tocs) == 0 || sv.parentSheet.parentChapter != me.Sheets[tocs[len(tocs)-1]].parentSheet.parentChapter {
			tocs = append(tocs, i)
		}
	}

	isforprint := (pgW == 0) && (pgH == 0)
	if !isforprint {
		s += `<g x="0" y="0">`
		faces := me.facesPicPaths()
		fperrow, fpercol := me.facesDistr(len(faces), float64(pgW), float64(pgH), false)
		s += me.facesDraw(faces, fperrow, fpercol, float64(pgW), float64(pgH), float64(pgW), float64(pgH), 0.0, 12.34, "px")
		s += `</g>`
	}

	hastoclist := os.Getenv("NOTOC") == "" && len(tocs) > 1
	s += `<g x="0" y="0">`
	s += `<text class="toctitle" x="` + ftoa(fIf(isforprint, 18.18, 31.13), -1) + `%" y="` + ftoa(fIf(hastoclist, 12.34, 54.32), -1) + `%"><tspan>` + os.Getenv("TITLE") + `</tspan></text>`
	if hastoclist {
		ypc, pstep := 22.0, (94.0-22.0)/float64(len(tocs)-1)
		for _, idx := range tocs {
			sv := me.Sheets[idx]
			chap := sv.parentSheet.parentChapter
			pgnr := iIf(isforprint, 5, 2) + idx/iIf(isforprint, 2, 1)
			s += `<text class="toc" x="8.88%" y="` + ftoa(ypc, -1) + `%"><tspan>` + itoa0pref(pgnr, 2) + sIf((pgnr >= 10 && pgnr < 20) || (((pgnr-1)%10) == 0), " ", "") + strings.Repeat("&#009;", iIf(pgnr >= 100, 3, 4)) + locStr(chap.Title, lang) + `</tspan></text>`
			if chap.author != nil {
				subtext, titleorig := "Story: ", chap.TitleOrig
				if prependWhen := false; prependWhen {
					_, dt := chap.dateRangeOfSheets(false, me.year)
					month1, month2 := "", dt.Month().String()
					if month := atoi(chap.Name[2:4], 0, 9999); month < 1 || month > 12 {
						panic(chap.Name[2:4])
					} else {
						month1 = time.Month(month).String()
					}
					if lang != App.Proj.Langs[0] {
						month1, month2 = App.Proj.textStr(lang, "Month_"+month1), App.Proj.textStr(lang, "Month_"+month2)
					}
					dtstr := month1[:3] + " 20" + chap.Name[:2] + " - " + month2[:3] + " " + itoa(me.year)
					if idx := strings.IndexByte(dtstr, '-'); idx > 0 && dtstr[:idx-1] == dtstr[idx+2:] {
						dtstr = dtstr[:idx-1]
					}
					subtext = "(" + dtstr + ")&#xA0;&#xA0;&#8212;&#xA0;&#xA0;" + subtext
				}
				if chap.TitleOrig == locStr(chap.Title, lang) {
					titleorig = ""
				} else if titleorig == "" && lang != App.Proj.Langs[0] {
					titleorig = locStr(chap.Title, App.Proj.Langs[0])
				}
				if titleorig != "" {
					subtext += "&quot;" + xEsc(titleorig) + "&quot;, "
				}
				subtext += xEsc("©") + itoa(chap.Year) + " " + chap.author.str(false, false)
				s += `<text class="tocsub" x="24%" y="` + ftoa(ypc+2.22, -1) + `%"><tspan>` + subtext + `</tspan></text>`
			}
			ypc += pstep
		}
	}
	s += `</g>`
	return
}

func (me *BookGen) facesPicPaths() []string {
	if me.facesFilePaths == nil {
		me.facesFilePaths = make([]string, 0, len(me.Sheets))
		for _, sv := range me.Sheets {
			var svimg *image.Gray
			var pidx int
			sv.data.PanelsTree.each(func(p *ImgPanel) {
				for i, area := range sv.panelFaceAreas(pidx) {
					rect, facefilepath := area.Rect, ".ccache/.pngtmp/face_"+sv.id+itoa0pref(pidx, 2)+itoa0pref(i, 2)+".png"
					if fileStat(facefilepath) == nil {
						if svimg == nil {
							if img, _, err := image.Decode(bytes.NewReader(fileRead(sv.data.bwFilePath))); err != nil {
								panic(err)
							} else {
								svimg = img.(*image.Gray)
							}
						}
						imgface := svimg.SubImage(rect).(*image.Gray)
						wh := iIf(rect.Dx() > rect.Dy(), rect.Dx(), rect.Dy())
						imgsq := image.NewGray(image.Rect(0, 0, wh, wh))
						imgFill(imgsq, imgsq.Bounds(), color.Gray{255})
						fx, fy := 0, 0
						if diff := rect.Dy() - rect.Dx(); diff > 0 {
							fx = diff / 2
						} else if diff := rect.Dx() - rect.Dy(); diff > 0 {
							fy = diff / 2
						}
						draw.Draw(imgsq, image.Rect(fx, fy, fx+rect.Dx(), fy+rect.Dy()), imgface, imgface.Bounds().Min, draw.Over)
						fileWrite(facefilepath, pngEncode(imgsq))
						pngOptFireAndForget(facefilepath)
					}
					me.facesFilePaths = append(me.facesFilePaths, absPath(facefilepath))
				}
				pidx++
			})
		}
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(me.facesFilePaths), func(i int, j int) {
			me.facesFilePaths[i], me.facesFilePaths[j] = me.facesFilePaths[j], me.facesFilePaths[i]
		})
	}
	return me.facesFilePaths
}

func (me *BookGen) facesDistr(numFaces int, w float64, h float64, double bool) (perRow int, perCol int) {
	perRow, perCol = 0, 3
	for numfaces := -1; numfaces < numFaces; {
		perCol++
		perRow = int(float64(perCol) / (h / w))
		numfaces = iIf(double, 2, 1) * perRow * perCol
	}
	return
}

func (me *BookGen) facesDraw(faces []string, perRow int, perCol int, areaWidth float64, areaHeight float64, svgWidth float64, svgHeight float64, spine float64, margin float64, svgUnit string) (svg string) {
	fpad, fwh, fy0 := (areaWidth/float64(perRow))/9.0, 0.0, 0.0
	for i := 0.0; fy0 < margin; i += 1.11 {
		fwh = ((areaWidth / float64(perRow)) - fpad) - i
		facesheight := svgHeight - (float64(perCol) * (fwh + fpad))
		fy0 = (0.5 * fpad) + (0.5 * facesheight)
	}
	faceswidth := (float64(perRow) * (fwh + fpad))
	fx, fy, fidx := margin+(0.5*fpad)+(0.5*(areaWidth-faceswidth)), fy0, 0
	isforscreen := (svgUnit == "px")
	if isforscreen {
		fx = 0.5 * (svgWidth - (faceswidth - fpad))
	}
	for first, doneincol := true, 0; true; first = false {
		if doneincol >= perCol {
			doneincol, fy, fx = 0, fy0, fx+fwh+fpad
			if center := (svgWidth * 0.5); spine > 0.01 && fx <= center && (fx+fwh+fpad) >= center {
				fx = spine + (0.33 * fpad) + (0.5 * (areaWidth - faceswidth))
			}
		} else if !first {
			fy += fwh + fpad
		}
		if bIf(isforscreen, (fx+fwh) > svgWidth, (fx+fwh+fpad) > (svgWidth-margin)) {
			break
		}
		svg += `<image x="` + ftoa(fx, -1) + svgUnit + `" y="` + ftoa(fy, -1) + svgUnit + `"
					width="` + ftoa(fwh, -1) + svgUnit + `"  height="` + ftoa(fwh, -1) + svgUnit + `"
					xlink:href="file://` + faces[fidx] + `" />`
		if fidx++; fidx >= len(faces) {
			fidx = 0
		}
		doneincol++
	}
	return
}

func (me *BookGen) genPrintCover(title string, numPages int) {
	marginmm := 22.0
	knownsizes := []struct {
		numPgs int
		mm     float64
	}{
		{0, 471.5},
		{52, 473},
		{68, 474.5},
		{88, 476},
		{108, 477},
		{132, 479},
		{156, 480.5},
		{180, 482},
		{204, 483.5},
		{228, 485},
	}

	outfilepathsvg := me.ShmDirPath + "/printcover.svg"
	printLn(outfilepathsvg, "...")

	faces := me.facesPicPaths()
	svgw, svgh := knownsizes[0].mm, 340.0
	for _, knownsize := range knownsizes[1:] {
		if numPages >= knownsize.numPgs {
			svgw = knownsize.mm
		}
	}
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
				width="` + ftoa(svgw-1, -1) + `mm" height="` + ftoa(svgh-1, -1) + `mm" style="background-color: #ffffff">
				<style type="text/css">
					@page { margin: 0; padding: 0; line-height: unset; size: ` + ftoa(svgw, -1) + `mm ` + ftoa(svgh, -1) + `mm; }
					* { margin: 0; padding: 0; line-height: unset; }
					text, text > tspan {
						font-family: "Shark Heavy ABC";
						font-size: 8.88mm;
						font-weight: normal;
						fill: #ffffff;
						writing-mode: vertical-lr;
						letter-spacing: -0.01em;
					}
				</style>`

	const spinemm = 22
	spinex := (svgw * 0.5) - (float64(spinemm) * 0.5)
	svg += `<rect fill="#000000" width="` + itoa(spinemm) + `mm" height="100%" y="0mm" x="` + ftoa(spinex, -1) + `mm" />`
	svg += `<text x="` + ftoa(0.5+(svgw*0.5), -1) + `mm" y="` + ftoa(svgh/3.0, -1) + `mm"><tspan>` + title + `</tspan></text>`

	areawidth, areaheight := spinex-marginmm, svgh-(marginmm*2.0)
	fperrow, fpercol := me.facesDistr(len(faces), areawidth, areaheight, true)
	svg += me.facesDraw(faces, fperrow, fpercol, areawidth, areaheight, svgw, svgh, spinex+float64(spinemm), marginmm, "mm")
	if os.Getenv("COVDBG") != "" { // debug rects (gray) for margins
		svg += `<rect opacity="0.5" fill="#cccccc" width="` + ftoa(svgw, -1) + `mm" height="` + ftoa(marginmm, -1) + `mm" y="0" x="0" />
				<rect opacity="0.5" fill="#cccccc" width="` + ftoa(marginmm, -1) + `mm" height="` + ftoa(svgh, -1) + `mm" y="0" x="0" />
				<rect opacity="0.5" fill="#cccccc" width="` + ftoa(svgw, -1) + `mm" height="` + ftoa(marginmm, -1) + `mm" y="` + ftoa(svgh-marginmm, -1) + `mm" x="0" />
				<rect opacity="0.5" fill="#cccccc" width="` + ftoa(marginmm, -1) + `mm" height="` + ftoa(svgh, -1) + `mm" y="0" x="` + ftoa(svgw-marginmm, -1) + `mm" />`
	}
	svg += "</svg>"

	fileWrite(outfilepathsvg, []byte(svg))
	if os.Getenv("NOPDF") == "" {
		me.printSvgToPdf(outfilepathsvg, me.OutDirPath+"/"+bookFileName(me.Phrase, "", "", false, ".pdf"))
	}
}

func (*BookGen) printSvgToPdf(svgFilePath string, pdfOutFilePath string) {
	printLn(pdfOutFilePath, "...")
	s := osExec(false, nil, browserCmd[0], append(browserCmd[2:],
		// "--headless",
		"--no-pdf-header-footer", "--disable-hang-monitor", // "--print-to-pdf="+pdfOutFilePath,
		svgFilePath)...)
	if fstat := fileStat(pdfOutFilePath); fstat == nil || fstat.Size() == 0 {
		panic(s)
	}
}

func bookFileName(bookName string, pref string, lang string, dirRtl bool, ext string) string {
	return App.Proj.Site.Host + "_" + bookName + "_" + sIf(pref == "", "printcover", pref+"_"+lang+`_`+sIf(dirRtl, "rtl", "ltr")) + ext
}
