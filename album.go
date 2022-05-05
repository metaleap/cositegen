package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	albumBookScreenWidth      = 4096
	albumBookScreenBorder     = 123
	albumBookPrintBorderMmBig = 15
	albumBookPrintBorderMmLil = 7
)

type AlbumBookGen struct {
	Sheets         []*SheetVer
	Phrase         string
	TmpDirPath     string
	OutDirPath     string
	MaxSheetWidth  int
	MaxSheetHeight int
}

func makeAlbumBook(flags map[string]struct{}) {
	phrase := strings.Join(os.Args[2:], " ")
	gen := AlbumBookGen{
		Phrase:     phrase,
		TmpDirPath: "/dev/shm/" + phrase,
		OutDirPath: ".books/" + phrase,
	}
	rmDir(gen.TmpDirPath)
	rmDir(gen.OutDirPath)
	mkDir(gen.TmpDirPath)
	mkDir(gen.OutDirPath)

	for k := range flags {
		y := atoi(k, 0, 99999)
		for _, series := range App.Proj.Series {
			for _, chap := range series.Chapters {
				if chap.Name == k || chap.UrlName == k ||
					series.Name == k || series.UrlName == k ||
					(y > 0 && chap.scanYearHas(y, true)) {
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
	}
	sort.SliceStable(gen.Sheets, func(i int, j int) bool {
		return gen.Sheets[i].DtStr() < gen.Sheets[j].DtStr()
	})
	if len(gen.Sheets) == 0 {
		panic("no scans found for: " + phrase)
	}

	for _, dirrtl := range []bool{false, true} {
		for _, lang := range App.Proj.Langs {
			gen.genSheetSvgsAndPngs(dirrtl, lang)
			if os.Getenv("NOSCREEN") == "" {
				gen.genScreenVersion(dirrtl, lang)
			}
			if os.Getenv("NOPRINT") == "" {
				numpages := gen.genPrintVersion(dirrtl, lang)
				if title := os.Getenv("TITLE"); title != "" {
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

func (me *AlbumBookGen) genSheetSvgsAndPngs(dirRtl bool, lang string) {
	for i, sv := range me.Sheets {
		sheetsvgfilepath := me.sheetSvgPath(i, dirRtl, lang)
		me.genSheetSvg(sv, sheetsvgfilepath, dirRtl, lang)
		if os.Getenv("NOSCREEN") == "" {
			sheetpngfilepath := sheetsvgfilepath + ".sh.png"
			printLn(filepath.Base(sheetpngfilepath), "...")
			imgAnyToPng(sheetsvgfilepath, sheetpngfilepath, 0, false, "sh_")
		}
	}
}

func (me *AlbumBookGen) genSheetSvg(sv *SheetVer, outFilePath string, dirRtl bool, lang string) {
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
		strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + `
				}
				g > svg > svg > text > tspan.std {
					stroke: #000000 !important;
					stroke-width: 11px !important;
				}
            </style>`

	pidx, qidx := 0, iIf(os.Getenv("LORES") == "", App.Proj.maxQualiIdx(), 0)

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

func (me *AlbumBookGen) sheetSvgPath(idx int, dirRtl bool, lang string) string {
	return me.TmpDirPath + "/" + sIf(dirRtl, "r", "l") + itoa0pref(idx, 3) + lang + ".svg"
}

func (me *AlbumBookGen) genScreenVersion(dirRtl bool, lang string) {
	pgw, pgh := albumBookScreenWidth, int(float64(albumBookScreenWidth)/(float64(me.MaxSheetWidth)/float64(me.MaxSheetHeight)))
	var pgfilepaths []string

	for i := range me.Sheets {
		outfilepath := me.sheetSvgPath(i, dirRtl, lang) + ".pg.png"
		printLn(filepath.Base(outfilepath), "...")
		imgpg := image.NewNRGBA(image.Rect(0, 0, pgw, pgh))
		for x := 0; x < pgw; x++ {
			for y := 0; y < pgh; y++ {
				imgpg.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}

		imgsh, _, err := image.Decode(bytes.NewReader(fileRead(me.sheetSvgPath(i, dirRtl, lang) + ".sh.png")))
		if err != nil {
			panic(err)
		}
		shw := pgw - (2 * albumBookScreenBorder)
		shh := int(float64(shw) / (float64(imgsh.Bounds().Dx()) / float64(imgsh.Bounds().Dy())))
		if shw > pgw || shh > pgh || shh > shw {
			panic("NEWBUG")
		}
		shx, shy := (pgw-shw)/2, (pgh-shh)/2
		ImgScaler.Scale(imgpg, image.Rect(shx, shy, shx+shw, shy+shh), imgsh, imgsh.Bounds(), draw.Over, nil)
		var buf bytes.Buffer
		if err = PngEncoder.Encode(&buf, imgpg); err != nil {
			panic(err)
		}
		fileWrite(outfilepath, buf.Bytes())
		pgfilepaths = append(pgfilepaths, outfilepath)
	}

	{
		outfilepathcbz := me.OutDirPath + "/screen_" + lang + sIf(dirRtl, "_rtl", "_ltr") + ".cbz"
		printLn(filepath.Base(outfilepathcbz), "...")
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

	{
		outfilepathpdf := me.OutDirPath + "/screen_" + lang + sIf(dirRtl, "_rtl", "_ltr") + ".pdf"
		printLn(filepath.Base(outfilepathpdf), "...")
		cmdArgs := []string{"--pillow-limit-break", "--nodate",
			"--pagesize", "A5^T"}
		cmdArgs = append(cmdArgs, pgfilepaths...)
		osExec(true, nil, "img2pdf", append(cmdArgs, "-o", outfilepathpdf)...)
	}
}

func (me *AlbumBookGen) genPrintVersion(dirRtl bool, lang string) (numPages int) {
	svgh, pgwmm, pghmm, isoddpage, pgidx, brepl := 0, 210, 297, false, -1, []byte("tspan.std")
	for numPages = 4 + len(me.Sheets)/2; (numPages % 4) != 0; {
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
						opacity="` + sIf(closeTag, "0.44", "0.22") + `" transform="rotate(` + itoa(iIf(even, 2, -2)) + `)" />`
		}
		if closeTag {
			svg += "</svg>"
		}
	}
	dpadd(true)
	dpadd(true)
	{
		dpadd(false)
		var tocs []int
		for i, sv := range me.Sheets {
			if len(tocs) == 0 || sv.parentSheet.parentChapter != me.Sheets[tocs[len(tocs)-1]].parentSheet.parentChapter {
				tocs = append(tocs, i)
			}
		}
		ypc := 10
		for _, idx := range tocs {
			sv := me.Sheets[idx]
			pgnr := 5 + idx/2
			svg += `<text class="toc" x="3cm" y="` + itoa(ypc) + `%"><tspan>` + itoa0pref(pgnr, 2) + "........" + locStr(sv.parentSheet.parentChapter.Title, lang) + `</tspan></text>`
			ypc += 10
		}
		svg += "</svg>"
	}
	dpadd(true)
	for i := 0; i < len(me.Sheets)/2; i++ {
		svgpgstart()
		sheetsvgfilepath0 := me.sheetSvgPath(i*2, dirRtl, lang)
		sheetsvgfilepath1 := me.sheetSvgPath((i*2)+1, dirRtl, lang)
		svg += `<text x="50%" y="97%"><tspan>` + itoa(pgidx+1) + `</tspan></text>`
		topborder := albumBookPrintBorderMmBig
		if me.Sheets[i*2].parentSheet.parentChapter.Name == "half-pagers" {
			topborder = albumBookPrintBorderMmLil
		}
		svg += `<image x="` + itoa(iIf(isoddpage, albumBookPrintBorderMmBig, albumBookPrintBorderMmLil)) + `mm" y="` + itoa(topborder) + `mm" width="` + itoa(pgwmm-(albumBookPrintBorderMmBig+albumBookPrintBorderMmLil)) + `mm" xlink:href="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString(bytes.Replace(fileRead(sheetsvgfilepath0), brepl, []byte("zzz"), -1)) + `"/>`
		svg += `<image x="` + itoa(iIf(isoddpage, albumBookPrintBorderMmBig, albumBookPrintBorderMmLil)) + `mm" y="50%" width="` + itoa(pgwmm-(albumBookPrintBorderMmBig+albumBookPrintBorderMmLil)) + `mm" xlink:href="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString(bytes.Replace(fileRead(sheetsvgfilepath1), brepl, []byte("zzz"), -1)) + `"/>`
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
		strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css["@font-face"], "; "), "'./", "'"+strings.TrimSuffix(os.Getenv("PWD"), "/")+"/site/files/", -1) +
		`}
					text, text > tspan { ` +
		strings.Join(App.Proj.Gen.PanelSvgText.Css[""], "; ") + `
						font-size: 1em;
					}
					text.toc tspan {
						font-family: "Shark Heavy ABC";
						font-size: 1.11cm;
						font-weight: normal;
						paint-order: stroke;
						stroke: #ffffff;
						stroke-width: 1mm;
					}
				</style>
		` + svg + "</svg>"

	outfilepathsvg := me.TmpDirPath + "/print_" + lang + sIf(dirRtl, "_rtl", "_ltr") + ".svg"
	fileWrite(outfilepathsvg, []byte(svg))
	if os.Getenv("NOPDF") == "" {
		outfilepathpdf := me.OutDirPath + "/print_" + lang + sIf(dirRtl, "_rtl", "_ltr") + ".pdf"
		printLn(filepath.Base(outfilepathpdf), "...")
		me.printSvgToPdf(outfilepathsvg, outfilepathpdf)
	}
	return
}

func (me *AlbumBookGen) genPrintCover(title string, numPages int) {
	knownsizes := []struct {
		n int
		f float64
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

	outfilepathsvg := me.TmpDirPath + "/printcover.svg"
	printLn(outfilepathsvg, "...")

	type FaceFile struct {
		absFilePath string
		width       int
		height      int
	}
	var faces []FaceFile
	for _, sv := range me.Sheets {
		var svimg *image.Gray
		var pidx int
		var buf bytes.Buffer
		sv.data.PanelsTree.iter(func(p *ImgPanel) {
			for i, area := range sv.panelFaceAreas(pidx) {
				facefilepath := ".ccache/.pngtmp/" + sv.id + itoa0pref(pidx, 2) + itoa0pref(i, 2) + ".png"
				if fileStat(facefilepath) == nil {
					if svimg == nil {
						if img, err := png.Decode(bytes.NewReader(fileRead(sv.data.bwFilePath))); err != nil {
							panic(err)
						} else {
							svimg = img.(*image.Gray)
						}
					}
					buf.Reset()
					if err := PngEncoder.Encode(&buf, svimg.SubImage(area.Rect)); err != nil {
						panic(err)
					}
					fileWrite(facefilepath, buf.Bytes())
				}
				faces = append(faces, FaceFile{absPath(facefilepath), area.Rect.Dx(), area.Rect.Dy()})
			}
			pidx++
		})
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(faces), func(i int, j int) {
		faces[i], faces[j] = faces[j], faces[i]
	})

	svgw, svgh := knownsizes[0].f, 340.0
	for _, knownsize := range knownsizes[1:] {
		if numPages >= knownsize.n {
			svgw = knownsize.f
		}
	}
	svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
	width="` + ftoa(svgw, -1) + `mm" height="` + ftoa(svgh, -1) + `mm">
	<style type="text/css">
		@page { margin: 0; padding: 0; line-height: unset; size: ` + ftoa(svgw, -1) + `mm ` + ftoa(svgh, -1) + `mm; }
		* { margin: 0; padding: 0; line-height: unset; }
		text, text > tspan {
			font-family: "Shark Heavy ABC";
			font-size: 1.11cm;
			font-weight: normal;
			fill: #ffffff;
		}
	</style>`
	fw, fx, fy, fidx := 22.0, -11.0, -22.0, 0
	for {
		if fx > svgw {
			fx, fy = -11.0, fy+44.0
		} else {
			fx += fw
		}
		if fx > svgw && fy > svgh {
			break
		}
		svg += `<image x="` + ftoa(fx, -1) + `mm" y="` + ftoa(fy, -1) + `mm" width="` + ftoa(fw, -1) + `mm"
					xlink:href="file://` + faces[fidx].absFilePath + `" />`
		if fidx++; fidx == len(faces) {
			fidx = 0
		}
	}
	svg += "</svg>"

	fileWrite(outfilepathsvg, []byte(svg))
	if os.Getenv("NOPDF") == "" {
		outfilepathpdf := me.OutDirPath + "/printcover.pdf"
		printLn(filepath.Base(outfilepathpdf), "...")
		me.printSvgToPdf(outfilepathsvg, outfilepathpdf)
	}
}

func (*AlbumBookGen) printSvgToPdf(svgFilePath string, pdfOutFilePath string) {
	osExec(false, nil, browserCmd[0], append(browserCmd[2:],
		"--headless", "--disable-gpu", "--print-to-pdf-no-header",
		"--print-to-pdf="+pdfOutFilePath, svgFilePath)...)
}
