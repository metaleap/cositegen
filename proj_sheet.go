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
	parentSheetVer  *SheetVer
	dirPath         string
	bwFilePath      string
	bwSmallFilePath string
	hasBgCol        bool

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
		printLn("\t\tPrep " + me.id + ": fully because no svData")
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
	me.data.dirPath = ".cache/" + me.id
	me.data.bwFilePath = filepath.Join(me.data.dirPath, "bw."+itoa(int(App.Proj.BwThreshold))+".png")
	me.data.bwSmallFilePath = filepath.Join(me.data.dirPath, "bwsmall."+itoa(int(App.Proj.BwThreshold))+"."+itoa(int(App.Proj.BwSmallWidth))+".png")
	mkDir(me.data.dirPath)

	didgraydistr := me.ensureGrayDistr(forceFullRedo || shouldsaveprojdata)
	didbwsheet := me.ensureBwSheetPngs(forceFullRedo)
	if didgraydistr || didbwsheet {
		printLn("\t\tPrep "+me.id+": did grayDistr / bwSheet:", didgraydistr, didbwsheet)
	}
	didpanels := me.ensurePanelsTree(forceFullRedo || didbwsheet || shouldsaveprojdata)
	if didpanels {
		printLn("\t\tPrep "+me.id+": did panelsTree:", didpanels)
	}
	didpnlpics := me.ensurePanelPics(forceFullRedo || didpanels)
	if didpnlpics {
		printLn("\t\tPrep "+me.id+": did panelPics:", didpnlpics)
	}

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
		} else if data := imgToMonochrome(file, file.Close, uint8(App.Proj.BwThreshold)); data != nil {
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
		if fileinfo, _ := direntry.Info(); (!direntry.IsDir()) &&
			strings.HasPrefix(direntry.Name(), "bg") && strings.HasSuffix(direntry.Name(), ".svg") &&
			(bgsrcfile == nil || fileinfo == nil || bgsrcfile.ModTime().UnixNano() > fileinfo.ModTime().UnixNano()) {
			_ = os.Remove(filepath.Join(me.data.dirPath, direntry.Name()))
		}
	}
	if bgsrcfile != nil {
		pidx, bgsvgsrc := 0, string(fileRead(bgsrcpath))
		me.data.hasBgCol = true
		me.data.PanelsTree.iter(func(p *ImgPanel) {
			gid, dstfilepath := "pnl"+itoa(pidx), filepath.Join(me.data.dirPath, "bg"+itoa(pidx)+".svg")
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
				if s == "" {
					panic(svg)
				}
				pw, ph := p.Rect.Max.X-p.Rect.Min.X, p.Rect.Max.Y-p.Rect.Min.Y
				s = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
						<svg width="` + itoa(pw) + `" height="` + itoa(ph) + `" viewbox="0 0 ` + itoa(pw) + ` ` + itoa(ph) + `" xmlns="http://www.w3.org/2000/svg">
						` + s
				fileWrite(dstfilepath, []byte(s+"</svg>"))
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
			if App.Proj.hasSvgQuali() {
				fileWrite(filepath.Join(me.data.PicDirPath(0), itoa(pidx)+".svg"),
					imgSubRectSvg(imgsrc.(*image.Gray), panel.Rect, int(me.data.PxCm*App.Proj.PanelBorderCm)))
			}
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
	} else if os.Getenv("REDO_BGS") != "" {
		_ = os.Remove(bgtmplsvgfilepath)
	}

	if pw, ph := itoa(me.data.PanelsTree.Rect.Max.X), itoa(me.data.PanelsTree.Rect.Max.Y); did || nil == fileStat(bgtmplsvgfilepath) {
		svg := `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
		<svg inkscape:version="1.1 (c68e22c387, 2021-05-23)"
			sodipodi:docname="drawing.svg"
			xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"
			xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd"
			xmlns="http://www.w3.org/2000/svg"
			xmlns:svg="http://www.w3.org/2000/svg"
			xmlns:xlink="http://www.w3.org/1999/xlink"
			width="` + pw + `" height="` + ph + `" viewBox="0 0 ` + pw + ` ` + ph + `">
		`
		pidx := 0
		me.data.PanelsTree.iter(func(p *ImgPanel) {
			rand.Seed(time.Now().UnixNano())
			r, g, b := 64+rand.Intn(160+(80-64)), 56+rand.Intn(160+(80-56)), 48+rand.Intn(160+(80-48))
			x, y, w, h := p.Rect.Min.X, p.Rect.Min.Y, p.Rect.Max.X-p.Rect.Min.X, p.Rect.Max.Y-p.Rect.Min.Y
			gid := "pnl" + itoa(pidx)
			svg += `<g id="` + gid + `" inkscape:label="` + gid + `" inkscape:groupmode="layer" transform="translate(` + itoa(x) + ` ` + itoa(y) + `)">`
			svg += `<rect x="0" y="0"  stroke="#000000" stroke-width="1"
				fill="` + fmt.Sprintf("#%X%X%X", r, g, b) + `"
				width="` + itoa(w) + `" height="` + itoa(h) + `"></rect>
			`
			svg += "</g>\n"
			pidx++
		})
		gid := "pnls_" + strings.Replace(filebasename, ".", "_", -1)
		svg += `<g id="` + gid + `" inkscape:label="` + gid + `" inkscape:groupmode="layer"><image x="0" y="0" width="` + pw + `" height="` + ph + `" xlink:href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(fileRead(me.data.bwSmallFilePath)) + `" /></g>`
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
