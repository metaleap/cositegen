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
	bookScreenWidth      = 3744
	bookScreenBorder     = 44
	bookScreenLoResDiv   = 4
	bookPrintBorderMmBig = 15
	bookPrintBorderMmLil = 7
)

type BookGen struct {
	Sheets         []*SheetVer
	Phrase         string
	ShmDirPath     string
	OutDirPath     string
	MaxSheetWidth  int
	MaxSheetHeight int

	facesFilePaths []string
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

	var year int
	for k := range flags {
		if y := atoi(k, 0, 9999); y > 2020 && y < 2929 {
			year = y
		}
	}
	for _, series := range App.Proj.Series {
		for _, chap := range series.Chapters {
			if (flags[chap.Name] || flags[chap.UrlName] ||
				flags[series.Name] || flags[series.UrlName]) &&
				(year == 0 || chap.scanYearHas(year, true)) {
				for _, sheet := range chap.sheets {
					sv := sheet.versions[0]
					gen.Sheets = append(gen.Sheets, sv)

					rect := sv.data.pxBounds()
					w, h := rect.Dx(), rect.Dy()
					if w > gen.MaxSheetWidth {
						gen.MaxSheetWidth = w
					}
					if h > gen.MaxSheetHeight {
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
	for _, dirrtl := range []bool{false, true} {
		for _, lang := range App.Proj.Langs {
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
		sheetsvgfilepath := me.sheetSvgPath(i, dirRtl, lang)
		me.genSheetSvg(sv, sheetsvgfilepath, dirRtl, lang)
		if os.Getenv("NOSCREEN") == "" {
			sheetpngfilepath := sheetsvgfilepath + ".sh.png"
			printLn(sheetpngfilepath, "...")
			imgAnyToPng(sheetsvgfilepath, sheetpngfilepath, iIf(!lores, 0, bookScreenWidth/bookScreenLoResDiv), false, sIf(!lores, "sh_", "sh_lq_"))
		}
	}
}

const bookCssTspanStd = "stroke: #000000 !important; stroke-width: 11px !important;"

func (me *BookGen) genSheetSvg(sv *SheetVer, outFilePath string, dirRtl bool, lang string) {
	rectinner, lores := sv.data.pxBounds(), (os.Getenv("LORES") != "")

	w, h := rectinner.Dx(), rectinner.Dy()

	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg
        xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
        width="` + itoa(w) + `" height="` + itoa(h) + `" viewBox="0 0 ` + itoa(w) + ` ` + itoa(h) + `">
            <style type="text/css">
                polygon { stroke: black; fill: white; }
                @font-face { ` +
		strings.Replace(strings.Join(App.Proj.Sheets.Panel.SvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) +
		` 		}
                g > svg > svg > text, g > svg > svg > text > tspan { ` +
		strings.Join(App.Proj.Sheets.Panel.SvgText.Css[""], "; ") + `
				}
				g > svg > svg > text > tspan.std {
					` + bookCssTspanStd + `
				}
            </style>`

	pidx, qidx := 0, iIf(!lores, App.Proj.maxQualiIdx(false), 0)

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
		svg += `<defs><clipPath id="c` + gid + `"><rect x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"></rect></clipPath></defs>`
		if panelbgpngsrcfilepath := filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".png"); fileStat(panelbgpngsrcfilepath) != nil {
			svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
						xlink:href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(fileRead(panelbgpngsrcfilepath)) + `" />`
		} else {
			svg += `<rect x="0" y="0" stroke="#000000" stroke-width="0" fill="#ffffff"
                        width="` + itoa(pw) + `" height="` + itoa(ph) + `"></rect>`
		}
		svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
					xlink:href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(fileRead(filepath.Join(sv.data.PicDirPath(App.Proj.Qualis[qidx].SizeHint), itoa(pidx)+".png"))) + `" />`
		if lang != "" {
			svg += sv.genTextSvgForPanel(pidx, p, lang, false, true)
		}
		svg += "\n</g>\n\n"
		pidx++
	})
	svg += `</svg>`
	fileWrite(outFilePath, []byte(svg))
}

func (me *BookGen) sheetSvgPath(idx int, dirRtl bool, lang string) string {
	return me.ShmDirPath + "/" + sIf(dirRtl, "r", "l") + itoa0pref(idx, 3) + lang + sIf(os.Getenv("LORES") == "", "", "_lq") + ".svg"
}

func (me *BookGen) genScreenVersion(dirRtl bool, lang string) {
	border, pgw, pgh, lores := bookScreenBorder, bookScreenWidth, int(float64(bookScreenWidth)/(float64(me.MaxSheetWidth)/float64(me.MaxSheetHeight))), (os.Getenv("LORES") != "")
	if lores {
		pgw, pgh, border = pgw/bookScreenLoResDiv, pgh/bookScreenLoResDiv, border/bookScreenLoResDiv
	}

	pgfilepaths := []string{}
	{
		tocfilepathsvg := me.ShmDirPath + "/0toc." + lang + sIf(!lores, "", "_lq") + ".svg"
		tocfilepathpng := tocfilepathsvg + ".png"
		if fileStat(tocfilepathpng) == nil {
			svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
				width="` + itoa(pgw) + `" height="` + itoa(pgh) + `">
				<style type="text/css">
					text.toc tspan {
						font-family: "Shark Heavy ABC";
						font-size: ` + sIf(!lores, "12", "3") + `em;
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
						font-size: ` + sIf(!lores, "4.44", "1.11") + `em;
					}
					image {
						opacity: 0.22;
					}
				</style>
			`
			svg += me.tocSvg(lang, pgw, pgh) + "</svg>"
			fileWrite(tocfilepathsvg, []byte(svg))
			printLn(tocfilepathpng, "...")
			imgAnyToPng(tocfilepathsvg, tocfilepathpng, 0, false, sIf(!lores, "toc_", "toc_lq_"))
		}
		pgfilepaths = append(pgfilepaths, tocfilepathpng)
	}

	for i := range me.Sheets {
		shfilepath := me.sheetSvgPath(i, dirRtl, lang) + ".sh.png"
		outfilepath := me.sheetSvgPath(i, dirRtl, lang) + ".pg.png"
		printLn(outfilepath, "...")
		tmpfilepath := ".ccache/.pngtmp/pgsh_" + itoa(border) + "_" + itoa(pgw) + "_" + contentHashStr(fileRead(shfilepath)) + ".png"
		if fileStat(tmpfilepath) == nil {
			imgpg := image.NewNRGBA(image.Rect(0, 0, pgw, pgh))
			imgFill(imgpg, imgpg.Bounds(), color.NRGBA{R: 255, G: 255, B: 255, A: 255})
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
			var buf bytes.Buffer
			if err = PngEncoder.Encode(&buf, imgpg); err != nil {
				panic(err)
			}
			fileWrite(tmpfilepath, buf.Bytes())
			if !lores {
				_ = osExec(false, nil, "pngbattle", tmpfilepath)
			}
		}
		fileLink(tmpfilepath, outfilepath)
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
			} else {
				io.Copy(fw, bytes.NewReader(fileRead(srcfilepath)))
			}
		}
		if err := zw.Close(); err != nil {
			panic(err)
		}
		_ = outfile.Sync()
	}

	if os.Getenv("NOPDF") == "" {
		outfilepathpdf := me.OutDirPath + "/" + bookFileName(me.Phrase, "screen", lang, dirRtl, ".pdf")
		printLn(outfilepathpdf, "...")
		cmdArgs := []string{"--pillow-limit-break", "--nodate",
			"--pagesize", "A5^T"}
		cmdArgs = append(cmdArgs, pgfilepaths...)
		osExec(true, nil, "img2pdf", append(cmdArgs, "-o", outfilepathpdf)...)
	}
}

func (me *BookGen) genPrintVersion(dirRtl bool, lang string) (numPages int) {
	svgh, pgwmm, pghmm, isoddpage, pgidx, brepl := 0, 210, 297, false, -1, []byte(bookCssTspanStd)
	for numPages = iIf(os.Getenv("NOTOC") == "", 4, 2) + len(me.Sheets)/2; (numPages % 4) != 0; {
		numPages++
	}
	svg := ""
	dpbwidx, dpbwfilepaths := 0, make([]string, len(me.Sheets))
	for i, sv := range me.Sheets {
		dpbwfilepaths[i] = absPath(sv.data.bwSmallFilePath)
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(dpbwfilepaths), func(i int, j int) {
		dpbwfilepaths[i], dpbwfilepaths[j] = dpbwfilepaths[j], dpbwfilepaths[i]
	})
	svgpgstart := func() {
		isoddpage = !isoddpage
		pgidx++
		y := float64(pgidx*pghmm) + (float64(1+pgidx) * 0.125)
		svg += `<svg class="pg" x="0" y="` + ftoa(y, -1) + `mm" width="` + itoa(pgwmm) + `mm" height="` + itoa(pghmm) + `mm">`
		svgh = int(y) + pghmm
	}
	dpadd := func(closeTag bool) {
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
	for i := 0; i < len(me.Sheets)/2; i++ {
		svgpgstart()
		sheetsvgfilepath0 := me.sheetSvgPath(i*2, dirRtl, lang)
		sheetsvgfilepath1 := me.sheetSvgPath((i*2)+1, dirRtl, lang)
		svg += `<text x="50%" y="97%"><tspan>` + itoa(pgidx+1) + `</tspan></text>`
		topborder := bookPrintBorderMmBig
		if me.Sheets[i*2].parentSheet.parentChapter.Name == "half-pagers" {
			topborder = bookPrintBorderMmLil
		}
		const repl = "font-weight: normal !important;"
		svg += `<image x="` + itoa(iIf(isoddpage, bookPrintBorderMmBig, bookPrintBorderMmLil)) + `mm" y="` + itoa(topborder) + `mm" width="` + itoa(pgwmm-(bookPrintBorderMmBig+bookPrintBorderMmLil)) + `mm" xlink:href="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString(bytes.Replace(fileRead(sheetsvgfilepath0), brepl, []byte(repl), -1)) + `"/>`
		svg += `<image x="` + itoa(iIf(isoddpage, bookPrintBorderMmBig, bookPrintBorderMmLil)) + `mm" y="` + itoa(iIf(strings.HasPrefix(me.Sheets[i*2].parentSheet.name, "01FROGF"), 47, 50)) + `%" width="` + itoa(pgwmm-(bookPrintBorderMmBig+bookPrintBorderMmLil)) + `mm" xlink:href="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString(bytes.Replace(fileRead(sheetsvgfilepath1), brepl, []byte(repl), -1)) + `"/>`
		svg += "</svg>"
	}
	dpadd(true)
	for (1 + pgidx) < numPages {
		dpadd(true)
	}

	svg = `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
				width="` + itoa(pgwmm) + `mm" height="` + itoa(svgh) + `mm">
				<style type="text/css">
					@page { margin: 0; padding: 0; line-height: unset; size: ` + itoa(pgwmm) + `mm ` + itoa(pghmm) + `mm; }
					* { margin: 0; padding: 0; line-height: unset; }
					svg.pg { page-break-after: always; break-after: always;}
					image { transform-origin: center; transform-box: fill-box; }
					@font-face { ` +
		strings.Replace(strings.Join(App.Proj.Sheets.Panel.SvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) +
		`}
					text, text > tspan { ` +
		strings.Join(App.Proj.Sheets.Panel.SvgText.Css[""], "; ") + `
						font-size: 1em;
					}
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
						font-size: 1.11em;
						stroke-width: 0.088em;
						stroke: #ffffff;
					}
				</style>
		` + svg + "</svg>"

	outfilepathsvg := me.ShmDirPath + "/print_" + lang + sIf(dirRtl, "_rtl", "_ltr") + ".svg"
	fileWrite(outfilepathsvg, []byte(svg))
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
	s += `<text class="toctitle" x="` + ftoa(fIf(isforprint, 18.18, 34.56), -1) + `%" y="` + ftoa(fIf(hastoclist, 12.34, 54.32), -1) + `%"><tspan>` + os.Getenv("TITLE") + `</tspan></text>`
	if hastoclist {
		ypc, pstep := 22.0, (94.0-22.0)/float64(len(tocs)-1)
		for _, idx := range tocs {
			sv := me.Sheets[idx]
			chap := sv.parentSheet.parentChapter
			pgnr := iIf(isforprint, 5, 2) + idx/iIf(isforprint, 2, 1)
			s += `<text class="toc" x="8.88%" y="` + ftoa(ypc, -1) + `%"><tspan>` + itoa0pref(pgnr, 2) + "&#009;&#009;&#009;&#009;" + locStr(chap.Title, lang) + `</tspan></text>`
			if chap.author != nil {
				subtext, titleorig := "Story: ", chap.TitleOrig
				if chap.TitleOrig == locStr(chap.Title, lang) {
					titleorig = ""
				} else if titleorig == "" && lang != App.Proj.Langs[0] {
					titleorig = locStr(chap.Title, App.Proj.Langs[0])
				}
				if titleorig != "" {
					subtext += "&quot;" + xEsc(titleorig) + "&quot;, "
				}
				subtext += xEsc("©") + itoa(chap.Year) + " " + chap.author.str(false, false)
				s += `<text class="tocsub" x="22%" y="` + ftoa(ypc+2.22, -1) + `%"><tspan>` + subtext + `</tspan></text>`
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
			var buf bytes.Buffer
			sv.data.PanelsTree.iter(func(p *ImgPanel) {
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
						buf.Reset()
						if err := PngEncoder.Encode(&buf, imgsq); err != nil {
							panic(err)
						}
						fileWrite(facefilepath, buf.Bytes())
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
	faceswidth := float64(perRow) * (fwh + fpad)
	fx, fy, fidx := margin+(0.5*fpad)+(0.5*(areaWidth-faceswidth)), fy0, 0
	for first, doneincol := true, 0; true; first = false {
		if doneincol >= perCol {
			doneincol, fy, fx = 0, fy0, fx+fwh+fpad
			if center := (svgWidth * 0.5); fx <= center && (fx+fwh+fpad) >= center && spine > 0.01 {
				fx = spine + (0.33 * fpad) + (0.5 * (areaWidth - faceswidth))
			}
		} else if !first {
			fy += fwh + fpad
		}
		if (fx + fwh + fpad) > (svgWidth - margin) {
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
	osExec(false, nil, browserCmd[0], append(browserCmd[2:],
		"--headless", "--disable-gpu", "--print-to-pdf-no-header",
		"--print-to-pdf="+pdfOutFilePath, svgFilePath)...)
}

func bookFileName(bookName string, pref string, lang string, dirRtl bool, ext string) string {
	return App.Proj.Site.Host + "_" + bookName + "_" + sIf(pref == "", "printcover", pref+"_"+lang+`_`+sIf(dirRtl, "rtl", "ltr")) + ext
}
