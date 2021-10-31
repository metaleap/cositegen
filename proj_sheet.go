package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Sheet struct {
	parentChapter *Chapter
	name          string
	versions      []*SheetVer
}

func (me *Sheet) At(i int) fmt.Stringer { return me.versions[i] }
func (me *Sheet) Len() int              { return len(me.versions) }
func (me *Sheet) String() string        { return me.name }

func (me *Sheet) versionNoOlderThanOrLatest(dt int64) *SheetVer {
	if dt > 0 {
		for i := len(me.versions) - 1; i > 0; i-- {
			if me.versions[i].dateTimeUnixNano >= dt {
				return me.versions[i]
			}
		}
	}
	return me.versions[0]
}

type SheetVerData struct {
	parentSheetVer  *SheetVer
	dirPath         string
	bwFilePath      string
	bwSmallFilePath string
	hasBgCol        bool

	PxCm       float64
	FontFactor float64   `json:",omitempty"`
	GrayDistr  []int     `json:",omitempty"`
	PanelsTree *ImgPanel `json:",omitempty"`
}

func (me *SheetVerData) PicDirPath(quali int) string {
	return filepath.Join(me.dirPath, "__panels__"+itoa(int(App.Proj.BwThresholds[0]))+"_"+ftoa(App.Proj.PanelBorderCm, -1)+"_"+itoa(quali))
}

type SheetVer struct {
	parentSheet      *Sheet
	id               string
	dateTimeUnixNano int64
	fileName         string
	data             *SheetVerData
	prep             struct {
		sync.Mutex
		done bool
	}
}

func (me *SheetVer) DtName() string {
	return strconv.FormatInt(me.dateTimeUnixNano, 10)
}

func (me *SheetVer) DtStr() string {
	return time.Unix(0, me.dateTimeUnixNano).Format("20060102")
}

func (me *SheetVer) String() string { return me.fileName }

func (me *SheetVer) ensurePrep(fromBgPrep bool, forceFullRedo bool) (didWork bool) {
	if !fromBgPrep {
		if me.prep.done {
			return
		}
		me.prep.Lock()
		defer func() { me.prep.done = true; me.prep.Unlock() }()
		if me.prep.done {
			return
		}
	}
	shouldsaveprojdata := forceFullRedo
	if me.data == nil {
		shouldsaveprojdata = true
		me.data = &SheetVerData{parentSheetVer: me, PxCm: 472.424242424} //1200dpi
		{
			pngdata := fileRead(me.fileName)
			if img, _, err := image.Decode(bytes.NewReader(pngdata)); err != nil {
				panic(err)
			} else if w := img.Bounds().Max.X; w < 10000 {
				me.data.PxCm *= 0.5 //600dpi
			}
		}
		App.Proj.data.Sv.ById[me.id] = me.data
	}
	me.data.dirPath = ".ccache/" + me.id
	me.data.bwFilePath = filepath.Join(me.data.dirPath, "bw."+itoa(int(App.Proj.BwThresholds[0]))+".png")
	me.data.bwSmallFilePath = filepath.Join(me.data.dirPath, "bwsmall."+itoa(int(App.Proj.BwThresholds[0]))+"."+itoa(int(App.Proj.BwSmallWidth))+".png")
	mkDir(me.data.dirPath)

	didgraydistr := me.ensureGrayDistr(forceFullRedo || shouldsaveprojdata)
	didbwsheet := me.ensureBwSheetPngs(forceFullRedo)
	didpanels := me.ensurePanelsTree(forceFullRedo || didbwsheet || shouldsaveprojdata)
	didpnlpics := me.ensurePanelPics(forceFullRedo || didpanels)

	if didWork = didgraydistr || didbwsheet || didpanels || didpnlpics; shouldsaveprojdata || didWork {
		App.Proj.save()
	}
	return
}

func (me *SheetVer) ensureBwSheetPngs(force bool) bool {
	var exist1, exist2 bool
	for fname, bptr := range map[string]*bool{me.data.bwFilePath: &exist1, me.data.bwSmallFilePath: &exist2} {
		*bptr = (fileStat(fname) != nil)
	}

	if force || !(exist1 && exist2) {
		rmDir(me.data.dirPath) // because BwThreshold or BwSmallWidth might have been..
		mkDir(me.data.dirPath) // ..changed and thus the file names: so rm stale ones.
		if file, err := os.Open(me.fileName); err != nil {
			panic(err)
		} else if data := imgToMonochrome(file, file.Close, uint8(App.Proj.BwThresholds[0])); data != nil {
			fileWrite(me.data.bwFilePath, data)
		} else if err = os.Symlink("../../../"+me.fileName, me.data.bwFilePath); err != nil {
			panic(err)
		}
		if file, err := os.Open(me.data.bwFilePath); err != nil {
			panic(err)
		} else if data := imgDownsized(file, file.Close, int(App.Proj.BwSmallWidth), true); data != nil {
			fileWrite(me.data.bwSmallFilePath, data)
		} else if err = os.Symlink(filepath.Base(me.data.bwFilePath), me.data.bwSmallFilePath); err != nil {
			panic(err)
		}
		return true
	}
	return false
}

func (me *SheetVer) ensurePanelPics(force bool) bool {
	numpanels, _ := me.panelCount()
	diritems, err := os.ReadDir(me.data.dirPath)
	bgsrcpath := strings.TrimSuffix(me.fileName, ".png") + ".svg"
	bgsrcfile := fileStat(bgsrcpath)
	for _, direntry := range diritems {
		if fileinfo, _ := direntry.Info(); (!direntry.IsDir()) && strings.HasPrefix(direntry.Name(), "bg") &&
			(strings.HasSuffix(direntry.Name(), ".png") || strings.HasSuffix(direntry.Name(), ".svg")) &&
			(bgsrcfile == nil || fileinfo == nil ||
				bgsrcfile.ModTime().UnixNano() > fileinfo.ModTime().UnixNano() ||
				!strings.HasSuffix(direntry.Name(), App.Proj.PanelBgFileExt)) {
			_ = os.Remove(filepath.Join(me.data.dirPath, direntry.Name()))
		}
	}
	if bgsrcfile != nil {
		pidx, bgsvgsrc := 0, string(fileRead(bgsrcpath))
		me.data.hasBgCol = true
		me.data.PanelsTree.iter(func(p *ImgPanel) {
			gid, dstfilepath := "pnl"+itoa(pidx), filepath.Join(me.data.dirPath, "bg"+itoa(pidx)+App.Proj.PanelBgFileExt)
			if s, svg := "", bgsvgsrc; force || (nil == fileStat(dstfilepath)) {
				_ = os.Remove(dstfilepath)
				if idx := strings.Index(svg, `id="`+gid+`"`); idx > 0 {
					svg = svg[idx+len(`id="`+gid+`"`):]
					if idx = strings.Index(svg, `id="pnl`); idx > 0 {
						svg = svg[:idx]
						if idx = strings.Index(svg, ">"); idx > 0 {
							svg = svg[idx+1:]
							if idx = strings.LastIndex(svg, "</g>"); idx > 0 {
								s = svg[:idx]
							}
						}
					}
				}
				if s != "" {
					scale := float64(App.Proj.BwSmallWidth) / float64(me.data.PanelsTree.Rect.Max.X)
					pw, ph := int(float64(p.Rect.Dx())*scale), int(float64(p.Rect.Dy())*scale)
					s = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
						<svg width="` + itoa(pw) + `" height="` + itoa(ph) + `" viewbox="0 0 ` + itoa(pw) + ` ` + itoa(ph) + `" xmlns="http://www.w3.org/2000/svg">` +
						strIf(App.Proj.PanelBgBlur == 0, "",
							`<filter id="leblur"><feGaussianBlur in="SourceGraphic" stdDeviation="`+itoa(App.Proj.PanelBgBlur)+`" /></filter>
							<style type="text/css">path { filter: url(#leblur); }</style>`) +
						s + "</svg>"

					switch App.Proj.PanelBgFileExt {
					case ".svg":
						fileWrite(dstfilepath, []byte(s))
					case ".png":
						tmpfilepath := "/dev/shm/" + me.id + "_bg" + itoa(pidx) + ".svg"
						fileWrite(tmpfilepath, []byte(s))
						out, err := exec.Command("convert", tmpfilepath, "-resize", itoa(int(100.0*App.Proj.PanelBgScaleIfPng))+"%", dstfilepath).CombinedOutput()
						_ = os.Remove(tmpfilepath)
						if s := trim(string(out)); err != nil {
							_ = os.Remove(dstfilepath)
							panic(err.Error() + ">>>>" + s + "<<<<")
						} else if len(s) != 0 {
							_ = os.Remove(dstfilepath)
							panic(s)
						}
					default:
						panic(App.Proj.PanelBgFileExt)
					}
				}
			}
			pidx++
		})
	}

	if err != nil {
		panic(err)
	}
	for _, quali := range App.Proj.Qualis {
		force = force || (nil == dirStat(me.data.PicDirPath(quali.SizeHint)))
	}
	for pidx, pngdir := 0, me.data.PicDirPath(App.Proj.Qualis[0].SizeHint); pidx < numpanels && !force; pidx++ {
		force = (nil == fileStat(filepath.Join(pngdir, itoa(pidx)+".png"))) ||
			(App.Proj.hasSvgQuali() && (nil == fileStat(filepath.Join(me.data.PicDirPath(0), itoa(pidx)+".svg"))))
	}
	for _, fileinfo := range diritems {
		if rm, name := force, fileinfo.Name(); fileinfo.IsDir() && strings.HasPrefix(name, "__panels__") {
			if got, qstr := false, name[strings.LastIndexByte(name, '_')+1:]; (!rm) && qstr != "" {
				if q, err := strconv.ParseUint(qstr, 10, 64); err == nil {
					for _, quali := range App.Proj.Qualis {
						got = (quali.SizeHint == int(q)) || got
					}
					rm = !got
				}
			}
			if rm {
				rmDir(filepath.Join(me.data.dirPath, name))
			}
		}
	}
	if !force {
		return false
	}

	for _, quali := range App.Proj.Qualis {
		mkDir(me.data.PicDirPath(quali.SizeHint))
	}
	srcimgfile, err := os.Open(me.data.bwFilePath)
	if err != nil {
		panic(err)
	}
	imgsrc, _, err := image.Decode(srcimgfile)
	if err != nil {
		panic(err)
	}
	_ = srcimgfile.Close()

	var pidx int
	var work sync.WaitGroup
	me.data.PanelsTree.iter(func(panel *ImgPanel) {
		work.Add(1)
		go func(pidx int) {
			defer work.Done()
			pw, ph, sw := panel.Rect.Dx(), panel.Rect.Dy(), me.data.PanelsTree.Rect.Dx()
			for _, quali := range App.Proj.Qualis {
				if quali.SizeHint == 0 {
					continue
				}
				width := float64(quali.SizeHint) / (float64(sw) / float64(pw))
				height := width / (float64(pw) / float64(ph))
				w, h := int(width), int(height)
				px1cm := me.data.PxCm / (float64(sw) / float64(quali.SizeHint))
				var wassamesize bool
				pngdata := imgSubRectPng(imgsrc.(*image.Gray), panel.Rect, &w, &h, int(px1cm*App.Proj.PanelBorderCm), true, &wassamesize)
				fileWrite(filepath.Join(me.data.PicDirPath(quali.SizeHint), itoa(pidx)+".png"), pngdata)
				if wassamesize {
					break
				}
			}
			if App.Proj.hasSvgQuali() {
				fileWrite(filepath.Join(me.data.PicDirPath(0), itoa(pidx)+".svg"),
					imgSubRectSvg(imgsrc.(*image.Gray), panel.Rect, int(me.data.PxCm*App.Proj.PanelBorderCm)))
			}
		}(pidx)
		pidx++
	})
	work.Wait()

	return true
}

func (me *SheetVer) ensureGrayDistr(force bool) bool {
	if force || len(me.data.GrayDistr) != App.Proj.NumColorDistrClusters {
		if file, err := os.Open(me.fileName); err != nil {
			panic(err)
		} else {
			me.data.GrayDistr = imgGrayDistrs(file, file.Close, App.Proj.NumColorDistrClusters)
		}
		return true
	}
	return false
}

func (me *SheetVer) sizeCm() (float64, float64) {
	return float64(me.data.PanelsTree.Rect.Max.X) / me.data.PxCm, float64(me.data.PanelsTree.Rect.Max.Y) / me.data.PxCm
}

func (me *SheetVer) cmToPx(fs ...float64) (ret []int) {
	for _, f := range fs {
		ret = append(ret, int(f*me.data.PxCm))
	}
	return
}

func (me *SheetVer) panelAt(x int, y int) (pnl *ImgPanel, idx int) {
	pidx := 0
	me.data.PanelsTree.iter(func(p *ImgPanel) {
		if pnl == nil && p.Rect.Min.X <= x && p.Rect.Max.X >= x &&
			p.Rect.Min.Y <= y && p.Rect.Max.Y >= y {
			pnl, idx = p, pidx
		}
		pidx++
	})
	return
}

func (me *SheetVer) panelMostCoveredBy(r image.Rectangle) (pnl *ImgPanel, idx int) {
	pidx, lastperc := 0, 0.0
	idx = -1
	me.data.PanelsTree.iter(func(p *ImgPanel) {
		if r.In(p.Rect) {
			pnl, idx, lastperc = p, pidx, 100.0
		} else if r.Overlaps(p.Rect) {
			overlap := r.Intersect(p.Rect)
			pw, ph := 100.0/(float64(p.Rect.Dx())/float64(overlap.Dx())), 100.0/(float64(p.Rect.Dy())/float64(overlap.Dy()))
			if perc := 0.5 * (pw + ph); lastperc < 100.0 && perc > lastperc {
				pnl, idx, lastperc = p, pidx, perc
			}
		}
		pidx++
	})
	return
}

func (me *SheetVer) ensurePanelsTree(force bool) (did bool) {
	filebasename := filepath.Base(me.fileName)
	bgtmplsvgfilename := strings.TrimSuffix(filebasename, ".png") + ".svg"
	bgtmplsvgfilepath := filepath.Join(me.data.dirPath, bgtmplsvgfilename)
	if did = force || me.data.PanelsTree == nil; did {
		_ = os.Remove(bgtmplsvgfilepath)
		if file, err := os.Open(me.data.bwFilePath); err != nil {
			panic(err)
		} else {
			imgpanel := imgPanels(file, file.Close)
			me.data.PanelsTree = &imgpanel
		}
	} else if os.Getenv("REDO_BGS") != "" {
		_ = os.Remove(bgtmplsvgfilepath)
	}

	scale := float64(App.Proj.BwSmallWidth) / float64(me.data.PanelsTree.Rect.Max.X)
	if pw, ph := int(scale*float64(me.data.PanelsTree.Rect.Max.X)), int(scale*float64(me.data.PanelsTree.Rect.Max.Y)); did || nil == fileStat(bgtmplsvgfilepath) {
		svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
		<svg inkscape:version="1.1 (c68e22c387, 2021-05-23)"
			sodipodi:docname="drawing.svg"
			xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"
			xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd"
			xmlns="http://www.w3.org/2000/svg"
			xmlns:svg="http://www.w3.org/2000/svg"
			xmlns:xlink="http://www.w3.org/1999/xlink"
			width="` + itoa(pw) + `" height="` + itoa(ph) + `" viewBox="0 0 ` + itoa(pw) + ` ` + itoa(ph) + `">
		`
		pidx := 0
		me.data.PanelsTree.iter(func(p *ImgPanel) {
			x, y, w, h := float64(p.Rect.Min.X)*scale, float64(p.Rect.Min.Y)*scale, float64(p.Rect.Dx())*scale, float64(p.Rect.Dy())*scale
			gid := "pnl" + itoa(pidx)
			svg += `<g id="` + gid + `" inkscape:label="` + gid + `" inkscape:groupmode="layer" transform="translate(` + itoa(int(x)) + ` ` + itoa(int(y)) + `)">`
			svg += `<rect x="0" y="0" stroke="#000000" stroke-width="0" fill="#f7f2f0"
				width="` + itoa(int(w)) + `" height="` + itoa(int(h)) + `"></rect>
			`
			svg += "</g>\n"
			pidx++
		})
		gid := "pnls_" + strings.Replace(filebasename, ".", "_", -1)
		svg += `<g id="` + gid + `" inkscape:label="` + gid + `" inkscape:groupmode="layer">`
		if pngembed := false; pngembed {
			svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `" xlink:href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(fileRead(me.data.bwSmallFilePath)) + `" />`
		} else {
			svg += `<image x="0" y="0" width="` + itoa(pw) + `" height="` + itoa(ph) + `" xlink:href="../../../` + me.data.bwSmallFilePath + `" />`
		}
		svg += `</g></svg>`
		fileWrite(bgtmplsvgfilepath, []byte(svg))
	}
	return
}

func (me *SheetVer) panelAreas(panelIdx int) []ImgPanelArea {
	if all := App.Proj.data.Sv.textRects[me.id]; len(all) > panelIdx {
		return all[panelIdx]
	}
	return nil
}

func (me *SheetVer) hasFaceAreas() (ret bool) {
	var pidx int
	me.data.PanelsTree.iter(func(p *ImgPanel) {
		ret = ret || len(me.panelFaceAreas(pidx)) > 0
		pidx++
	})
	return
}

func (me *SheetVer) panelFaceAreas(panelIdx int) (ret []ImgPanelArea) {
	for _, area := range me.panelAreas(panelIdx) {
		if area.Rect.Dx() > 0 && area.Rect.Dy() > 0 {
			var hastext bool
			for _, v := range area.Data {
				if hastext = (v != ""); hastext {
					break
				}
			}
			if !hastext {
				ret = append(ret, area)
			}
		}
	}
	return
}

func (me *SheetVer) panelCount() (numPanels int, numPanelAreas int) {
	for _, areas := range App.Proj.data.Sv.textRects[me.id] {
		numPanels, numPanelAreas = numPanels+1, numPanelAreas+len(areas)
	}
	if numPanels == 0 && me.data != nil && me.data.PanelsTree != nil {
		me.data.PanelsTree.iter(func(p *ImgPanel) {
			numPanels++
		})
	}
	return
}

func (me *SheetVer) haveAnyTexts() bool {
	for _, areas := range App.Proj.data.Sv.textRects[me.id] {
		for _, area := range areas {
			for _, text := range area.Data {
				if trim(text) != "" {
					return true
				}
			}
		}
	}
	return false
}

func (me *SheetVer) maxNumTextAreas() (max int) {
	for _, panel := range App.Proj.data.Sv.textRects[me.id] {
		if l := len(panel); l > max {
			max = l
		}
	}
	return
}

func (me *SheetVer) grayDistrs() (r [][3]float64) {
	if me.data == nil || len(me.data.GrayDistr) == 0 {
		return nil
	}
	numpx, m := 0, 256.0/float64(len(me.data.GrayDistr))
	for _, cd := range me.data.GrayDistr {
		numpx += cd
	}
	for i, cd := range me.data.GrayDistr {
		r = append(r, [3]float64{float64(i) * m, float64(i+1) * m,
			1.0 / (float64(numpx) / float64(cd))})
	}
	return
}

func (me *SheetVer) genTextSvgForPanel(panelIdx int, panel *ImgPanel, lang string, forHtml bool) string {
	panelareas := me.panelAreas(panelIdx)
	if len(panelareas) == 0 {
		return ""
	}

	pw, ph := panel.Rect.Dx(), panel.Rect.Dy()
	s := "<svg viewbox='0 0 " + itoa(pw) + " " + itoa(ph) + "'>"
	for _, pta := range panelareas {
		rx, ry, rw, rh := pta.Rect.Min.X-panel.Rect.Min.X, pta.Rect.Min.Y-panel.Rect.Min.Y, pta.Rect.Dx(), pta.Rect.Dy()
		borderandfill := pta.PointTo != nil
		if borderandfill {
			rpx, rpy := pta.PointTo.X-panel.Rect.Min.X, pta.PointTo.Y-panel.Rect.Min.Y
			mmh, cmh := int(me.data.PxCm*App.Proj.Gen.PanelSvgText.BoxPolyStrokeWidthCm), int(me.data.PxCm/2.0)
			pl, pr, pt, pb := (rx + mmh), ((rx + rw) - mmh), (ry + mmh), ((ry + rh) - mmh)
			poly := [][2]int{{pl, pt}, {pr, pt}, {pr, pb}, {pl, pb}}
			ins := func(idx int, pts ...[2]int) {
				head, tail := poly[:idx], poly[idx:]
				poly = append(head, append(pts, tail...)...)
			}

			if !(pta.PointTo.X == 0 && pta.PointTo.Y == 0) { // "speech-text" pointing somewhere?
				dx, dy := intAbs(rpx-(rx+(rw/2))), intAbs(rpy-(ry+(rh/2)))
				isr, isb := rpx > (rx+(rw/2)), rpy > (ry+(rh/2))
				isl, ist, dst := !isr, !isb, [2]int{rpx, rpy}

				isbr := isb && isr && dy > dx
				isbl := isb && isl && dy > dx
				istr := ist && isr && dy > dx
				istl := ist && isl && dy > dx
				isrb := isr && isb && dx > dy && !isbr
				islb := isl && isb && dx > dy
				isrt := isr && ist && dx > dy
				islt := isl && ist && dx > dy

				if isbl || islb {
					ins(3, [2]int{pl + cmh, pb}, dst)
				} else if isbr || isrb {
					ins(3, dst, [2]int{pr - cmh, pb})
				} else if istr {
					ins(1, [2]int{pr - cmh, pt}, dst)
				} else if istl {
					ins(1, dst, [2]int{pl + cmh, pt})
				} else if isrt {
					ins(2, dst, [2]int{pr, pt + cmh})
				} else if isrb {
					ins(2, [2]int{pr, pb - cmh}, dst)
				} else if islt {
					ins(4, [2]int{pl, pt + cmh}, dst)
				} else if islb {
					ins(4, dst, [2]int{pl, pb - cmh})
				}
			}

			s += "<polygon points='"
			for _, pt := range poly {
				s += itoa(pt[0]) + "," + itoa(pt[1]) + " "
			}
			s += "' class='" + App.Proj.Gen.PanelSvgText.ClsBoxPoly + "' stroke-width='" + itoa(mmh) + "px'/>"
		}
		s += "<svg x='" + itoa(rx) + "' y='" + itoa(ry) + "'>" +
			me.genTextSvgForPanelArea(&pta, lang, forHtml) + "</svg>"
	}

	s += "</svg>"
	return s
}

func (me *SheetVer) genTextSvgForPanelArea(pta *ImgPanelArea, lang string, forHtml bool) string {
	linex := 0.0
	if pta.PointTo != nil {
		linex = me.data.PxCm * App.Proj.Gen.PanelSvgText.BoxPolyDxCmA4
	}
	fontSizeCmA4, perLineDyCmA4 := App.Proj.Gen.PanelSvgText.FontSizeCmA4, App.Proj.Gen.PanelSvgText.PerLineDyCmA4
	if me.parentSheet.parentChapter.GenPanelSvgText.FontSizeCmA4 > 0.01 { // !=0 in float
		fontSizeCmA4 = me.parentSheet.parentChapter.GenPanelSvgText.FontSizeCmA4
	}
	if me.parentSheet.parentChapter.GenPanelSvgText.PerLineDyCmA4 > 0.01 { // !=0 in float
		perLineDyCmA4 = me.parentSheet.parentChapter.GenPanelSvgText.PerLineDyCmA4
	}
	if me.data.FontFactor > 0.01 {
		fontSizeCmA4 *= me.data.FontFactor
		perLineDyCmA4 *= me.data.FontFactor
	}
	return imgSvgText(pta, lang, me.data.PxCm, int(linex), fontSizeCmA4, perLineDyCmA4, forHtml)
}
