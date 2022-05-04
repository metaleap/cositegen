package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

const (
	albumBookScreenWidth  = 4096
	albumBookScreenBorder = 123
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
	if len(gen.Sheets) == 0 {
		panic("no scans found for: " + phrase)
	}

	for _, dirrtl := range []bool{false, true} {
		gen.genSheetSvgsAndPngs(dirrtl)
		for _, lang := range App.Proj.Langs {
			gen.genAlbumFilesForScreen(dirrtl, lang)
			gen.genAlbumFilesForPrint(dirrtl, lang)
			if os.Getenv("NOLANG") != "" {
				break
			}
		}
		if os.Getenv("NORTL") != "" || os.Getenv("NODIR") != "" {
			break
		}
	}
}

func (me *AlbumBookGen) genSheetSvgsAndPngs(dirRtl bool) {
	for i, sv := range me.Sheets {
		for _, forPrint := range []bool{false, true} {
			for _, lang := range App.Proj.Langs {
				sheetsvgfilepath := me.sheetSvgPath(i, dirRtl, sIf(forPrint, "", lang))
				me.genSheetSvg(sv, sheetsvgfilepath, dirRtl, sIf(forPrint, "", lang))
				sheetpngfilepath := sheetsvgfilepath + ".sh.png"
				printLn(sheetpngfilepath)
				imgAnyToPng(sheetsvgfilepath, sheetpngfilepath, 0, false, sIf(forPrint, "shp_", "shs_"))
				if forPrint || os.Getenv("NOLANG") != "" {
					break
				}
			}
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
		if panelbgpngsrcfilepath := absPath(filepath.Join(sv.data.dirPath, "bg"+itoa(pidx)+".png")); fileStat(panelbgpngsrcfilepath) != nil {
			svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
                        xlink:href="` + panelbgpngsrcfilepath + `" />`
		} else {
			svg += `<rect x="0" y="0" stroke="#000000" stroke-width="0" fill="#ffffff"
                        width="` + itoa(pw) + `" height="` + itoa(ph) + `"></rect>`
		}
		panelpngsrcfilepath := absPath(filepath.Join(sv.data.PicDirPath(App.Proj.Qualis[App.Proj.maxQualiIdx()].SizeHint), itoa(pidx)+".png"))
		svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `"
                    xlink:href="` + panelpngsrcfilepath + `" />`
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

func (me *AlbumBookGen) genAlbumFilesForScreen(dirRtl bool, lang string) {
	pgw, pgh := albumBookScreenWidth, int(float64(albumBookScreenWidth)/(float64(me.MaxSheetWidth)/float64(me.MaxSheetHeight)))

	for i := range me.Sheets {
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
		fileWrite(me.sheetSvgPath(i, dirRtl, lang)+".pg.png", buf.Bytes())
	}
}

func (me *AlbumBookGen) genAlbumFilesForPrint(dirRtl bool, lang string) {

}
