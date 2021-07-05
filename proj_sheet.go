package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/png"
	"math/rand"
	"os"
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
	dirPath         string
	bwFilePath      string
	bwSmallFilePath string

	PxCm       float64
	GrayDistr  []int     `json:",omitempty"`
	PanelsTree *ImgPanel `json:",omitempty"`
}

func (me *SheetVerData) PicDirPath(quali int) string {
	return filepath.Join(me.dirPath, "__panels__"+itoa(int(App.Proj.BwThreshold))+"_"+ftoa(App.Proj.PanelBorderCm, -1)+"_"+itoa(quali))
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
		me.data = &SheetVerData{PxCm: 472.424242424} //1200dpi
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
	me.data.dirPath = ".cache/" + me.id
	me.data.bwFilePath = filepath.Join(me.data.dirPath, "bw."+itoa(int(App.Proj.BwThreshold))+".png")
	me.data.bwSmallFilePath = filepath.Join(me.data.dirPath, "bwsmall."+itoa(int(App.Proj.BwThreshold))+"."+itoa(int(App.Proj.BwSmallWidth))+".png")
	mkDir(me.data.dirPath)

	didgraydistr := me.ensureGrayDistr(forceFullRedo || shouldsaveprojdata)
	didbwsheet := me.ensureBwSheetPngs(forceFullRedo)
	didpanels := me.ensurePanelsTree(forceFullRedo || didbwsheet || shouldsaveprojdata)
	didpnlpics := me.ensurePanelPics(forceFullRedo || didpanels)

	if didWork = didgraydistr || didbwsheet || didpanels || didpnlpics; shouldsaveprojdata {
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
		} else if data := imgToMonochrome(file, file.Close, uint8(App.Proj.BwThreshold)); data != nil {
			fileWrite(me.data.bwFilePath, data)
		} else if err = os.Symlink("../../../"+me.fileName, me.data.bwFilePath); err != nil {
			panic(err)
		}
		if file, err := os.Open(me.data.bwFilePath); err != nil {
			panic(err)
		} else if data := imgDownsized(file, file.Close, int(App.Proj.BwSmallWidth), true); data != nil {
			fileWrite(me.data.bwSmallFilePath, data)
			pngOpt(me.data.bwSmallFilePath)
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
	if bgsrcpath := strings.TrimSuffix(me.fileName, ".png") + ".svg"; nil == fileStat(bgsrcpath) {
		for _, fileinfo := range diritems {
			if (!fileinfo.IsDir()) && strings.HasPrefix(fileinfo.Name(), "bg") && strings.HasSuffix(fileinfo.Name(), ".svg") {
				_ = os.Remove(filepath.Join(me.data.dirPath, fileinfo.Name()))
			}
		}
	} else {
		pidx, bgsvgsrc := 0, string(fileRead(bgsrcpath))
		me.data.PanelsTree.iter(func(p *ImgPanel) {
			dstfilepath := filepath.Join(me.data.dirPath, "bg"+itoa(pidx)+".svg")
			if svg := bgsvgsrc; force || (nil == fileStat(dstfilepath)) {
				_ = os.Remove(dstfilepath)
				if idx := strings.Index(svg, `id="p`+itoa(pidx)+`"`); idx > 0 {
					svg = svg[idx:]
					if idx = strings.Index(svg, "<"); idx > 0 {
						svg = svg[idx:]
						if idx = strings.Index(svg, "</svg>"); idx > 0 {
							svg = svg[:idx+len("</svg>")]
							pw, ph := p.Rect.Max.X-p.Rect.Min.X, p.Rect.Max.Y-p.Rect.Min.Y
							fileWrite(dstfilepath, []byte(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
								<svg xmlns="http://www.w3.org/2000/svg" viewbox="0 0 `+itoa(pw)+` `+itoa(ph)+`"
									width="`+itoa(pw)+`" height="`+itoa(ph)+`">`+
								svg),
							)
						}
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
			(nil == fileStat(filepath.Join(me.data.PicDirPath(0), itoa(pidx)+".svg")))
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
			pw, ph, sw := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y, me.data.PanelsTree.Rect.Max.X-me.data.PanelsTree.Rect.Min.X
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
			fileWrite(filepath.Join(me.data.PicDirPath(0), itoa(pidx)+".svg"),
				imgSubRectSvg(imgsrc.(*image.Gray), panel.Rect, int(me.data.PxCm*App.Proj.PanelBorderCm)))
			work.Done()
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
	}

	_ = os.Remove(bgtmplsvgfilepath)
	if pw, ph := itoa(me.data.PanelsTree.Rect.Max.X), itoa(me.data.PanelsTree.Rect.Max.Y); did || nil == fileStat(bgtmplsvgfilepath) {
		svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
		<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="` + pw + `" height="` + ph + `" viewBox="0 0 ` + pw + ` ` + ph + `">
		`
		pidx := 0
		me.data.PanelsTree.iter(func(p *ImgPanel) {
			rand.Seed(time.Now().UnixNano())
			r, g, b := 48+rand.Intn(160+(64-48)), 32+rand.Intn(160+(64-32)), 64+rand.Intn(160+(64-64))
			x, y, w, h := p.Rect.Min.X, p.Rect.Min.Y, p.Rect.Max.X-p.Rect.Min.X, p.Rect.Max.Y-p.Rect.Min.Y
			svg += `<svg x="` + itoa(x) + `" y="` + itoa(y) + `"  width="` + itoa(w) + `" height="` + itoa(h) + `" id="p` + itoa(pidx) + `">`
			svg += `<rect x="0" y="0"
				fill="` + fmt.Sprintf("#%X%X%X", r, g, b) + `"  stroke="#000000"
				stroke-width="1" width="` + itoa(w) + `" height="` + itoa(h) + `"></rect>
			`
			if true {
				svg += `<rect stroke-width="22" stroke="#ffcc00" x="` + itoa(w/2) + `" y="0" width="22" height="` + itoa(h) + `"></rect>`
				svg += `<rect stroke-width="22" stroke="#ffcc00" x="0" y="` + itoa(h/2) + `" width="` + itoa(w) + `" height="22"></rect>`
			}
			svg += "</svg>\n"
			pidx++
		})
		svg += `<image x="0" y="0" width="` + pw + `" height="` + ph + `" xlink:href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(fileRead(me.data.bwSmallFilePath)) + `" />`
		fileWrite(bgtmplsvgfilepath, []byte(svg+"</svg>"))
	}
	return
}

func (me *SheetVer) panelAreas(panelIdx int) []ImgPanelArea {
	if all := App.Proj.data.Sv.textRects[me.id]; len(all) > panelIdx {
		return all[panelIdx]
	}
	return nil
}

func (me *SheetVer) panelCount() (numPanels int, numPanelAreas int) {
	all := App.Proj.data.Sv.textRects[me.id]
	numPanels = len(all)
	for _, areas := range all {
		numPanelAreas += len(areas)
	}
	if numPanels == 0 && numPanelAreas == 0 && me.data != nil && me.data.PanelsTree != nil {
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
