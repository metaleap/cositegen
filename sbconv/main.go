package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/metaleap/cositegen/sb_shared"
)

const a5factor = 0.297 / 0.210

var itoa = strconv.Itoa

func ftoa(f float64, prec int) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(f, 'f', prec, 64), "0"), ".")
}

func main() {
	srcfilepaths := os.Args[1:]

	if codir := "/home/_/co"; len(srcfilepaths) == 0 {
		if err := fs.WalkDir(os.DirFS(codir), ".", func(path string, dir fs.DirEntry, err error) error {
			if strings.HasSuffix(path, "/storyboard.fodp") {
				srcfilepaths = append(srcfilepaths, filepath.Join(codir, path))
			}
			return err
		}); err != nil {
			panic(err)
		}
	}

	for _, srcfilepath := range srcfilepaths {
		if len(srcfilepaths) > 0 {
			println(srcfilepath, "...")
		}
		convert(srcfilepath)
	}
}

func convert(srcFilePath string) {
	src, err := os.ReadFile(srcFilePath)
	if err != nil {
		panic(err)
	}

	var sb Storyboard
	for _, xmlpage := range xmlOuters(string(src), "<draw:page>", "</draw:page>") {
		page := Page{Name: xmlAttr(xmlpage, "draw:name")}
		for _, xmlframe := range xmlOuters(xmlpage, "<draw:frame>", "</draw:frame>") {
			if xmlAttr(xmlframe, "presentation:style-name") != "" {
				continue
			}
			var balloon Object
			setSizeAndPosFrom(&balloon.SizeAndPos, xmlframe)
			objSetParasFrom(&balloon, xmlframe)
			page.Balloons = append(page.Balloons, balloon)
		}
		for _, xmlshape := range xmlOuters(xmlpage, "<draw:custom-shape>", "</draw:custom-shape>") {
			var panel Object
			setSizeAndPosFrom(&panel.SizeAndPos, xmlshape)
			objSetParasFrom(&panel, xmlshape)
			page.Panels = append(page.Panels, panel)
		}
		sb = append(sb, page)
	}

	jsonfilepath := srcFilePath[:len(srcFilePath)-len(".fodp")] + ".json"
	_ = os.Remove(jsonfilepath)
	if err := os.WriteFile(jsonfilepath, sbToJson(sb), os.ModePerm); err != nil {
		panic(err)
	}

	for _, pgsize := range []string{"A4", "A5", "A3"} {
		htmlfilepath := srcFilePath[:len(srcFilePath)-len(".fodp")] + "." + pgsize + ".html"
		pdffilepath := srcFilePath[:len(srcFilePath)-len(".fodp")] + "." + pgsize + ".pdf"
		_ = os.Remove(htmlfilepath)
		_ = os.Remove(pdffilepath)
		if err := os.WriteFile(htmlfilepath, sbToHtml(sb, srcFilePath, pgsize == "A5", pgsize == "A3"), os.ModePerm); err != nil {
			panic(err)
		}
		html2pdf, err := exec.Command("wkhtmltopdf",
			"--orientation", "Landscape",
			"--page-size", pgsize,
			"--log-level", "error",
			"--grayscale", htmlfilepath, pdffilepath,
		).CombinedOutput()
		if h2p := strings.TrimSpace(string(html2pdf)); h2p != "" {
			println(h2p)
		}
		if err != nil {
			panic(err)
		}
	}
}

func sbToJson(it Storyboard) []byte {
	data, err := json.MarshalIndent(it, "", "  ")
	if err != nil {
		panic(err)
	}
	return data
}

func sbToHtml(it Storyboard, srcFilePath string, isA5 bool, isA3 bool) []byte {
	title, zoom := filepath.Dir(srcFilePath), 125
	if title == "" || title == "." {
		title = filepath.Base(os.Getenv("PWD"))
	}
	if isA5 {
		zoom = 87
	} else if isA3 {
		zoom = 177
	}
	s := `<!DOCTYPE html><html lang="{{.PageLang}}" dir="{{.PageDirCur}}"><head>
			<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta content="text/html;charset=utf-8" http-equiv="Content-Type" />
			<title>` + title + `</title><style type="text/css">
				body {
					zoom: ` + itoa(zoom) + `%;
					padding: 0 0cm;
					margin: 0px;
				}
				.box {
					background-color: #ffffff;
					opacity: 77%;
					margin: 0px;
					padding: 0px;
					box-sizing: border-box;
					border: 0.05cm solid #000000;
					border-collapse: collapse;
					overflow: hidden;
					text-overflow: clip;
				}
				.page {
					position: relative !important;
					page-break-after: always;
					width: 29.7cm;
					max-width: 29.7cm;
					min-width: 29.7cm;
					height: 21cm;
					max-height: 21cm;
					min-height: 21cm;
					border: 0px none #000 !important;
					margin-bottom: 0cm;
				}
				.obj {
					position: absolute;
					display: flex;
					align-items: center;
					align-content: center;
					text-align: center;
				}
				.obj > div, .obj > div > p {
					margin: 0px;
					padding: 0px;
					width: 99%;
					text-align: center;
				}
				p {
					font-weight: bold;
					font-size: 18pt;
					line-height: 0.88em;
					font-family: "Annie Use Your Telescope", sans;
				}
				.panel {
					align-items: end;
					align-content: end;
					color: #505050;
				}
				.panel, .panel p {
					text-align: right !important;
				}
				.balloon {
					border-radius: 1em;
				}
				.balloon p span {
					font-size: 0.88em;
				}
				.balloon p {
					line-height: 0.77em;
				}
				hr {
					opacity: 0.22;
					margin: 1em 0;
				}
				small {
					font-weight: normal;
					font-family: mono;
					font-size: small;
					letter-spacing: -0.22em;
				}
			</style></head><body>`
	for _, page := range it {
		if len(page.Panels) == 0 && len(page.Balloons) == 0 {
			continue
		}
		s += `<div title="` + page.Name + `" class="page box">`
		for _, p := range page.Panels {
			s += objToHtml(&p, "panel", 123, isA5, isA3)
		}
		for idx, b := range page.Balloons {
			var prior *Object
			for _, bp := range page.Balloons[:idx] {
				if bp.CmH == b.CmH && bp.CmY == b.CmY &&
					bp.CmX >= b.CmX && bp.CmW <= b.CmW &&
					(bp.CmX+bp.CmW) <= (b.CmX+b.CmW) {
					prior = &bp
					break
				}
			}
			if prior != nil {
				b.Paras = prior.Paras
			}
			s += objToHtml(&b, "balloon", 1, isA5, isA3)
		}
		s += `</div>`
	}
	s += "</body></html>"
	return []byte(s)
}

func objToHtml(it *Object, cssClsExtra string, repeatParas int, isA5 bool, isA3 bool) (s string) {
	cmw, cmh := it.CmW, it.CmH
	if isA5 {
		cmw, cmh = cmw/a5factor, cmh/a5factor
	} else if isA3 {
		cmw, cmh = cmw*a5factor, cmh*a5factor
	}
	strcm := ftoa(cmw, 1) + `&bull;` + ftoa(cmh, 1)

	s += `<div title="(` + strcm + "')\n" + strings.Join(it.Paras, "\n") + `" class="obj box ` + cssClsExtra + `" style="left: ` + ftoa(it.CmX, 9) + `cm; top: ` + ftoa(it.CmY, 9) + `cm; width: ` + ftoa(it.CmW, 9) + `cm; height: ` + ftoa(it.CmH, 9) + `cm"><div>`
	for i := 0; i < repeatParas; i++ {
		for j, para := range it.Paras {
			para = "<span>" + para + "</span>"
			if j == 0 {
				para = `<sup><small>` + strcm + `&nbsp;</small></sup>` + para
			}
			s += `<p>` + para + `</p>`
		}
		if repeatParas > 1 {
			s += "<hr/>"
		}
	}
	s += `</div></div>`
	return s
}
