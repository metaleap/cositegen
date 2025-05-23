package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const dpi1200 float64 = 472.424242424
const svCacheDirNamePrefix = "" // TODO: switch from empty to sth like "sv"

type Sheet struct {
	parentChapter *Chapter
	name          string
	versions      []*SheetVer
}

func (me *Sheet) At(i int) fmt.Stringer { return me.versions[i] }
func (me *Sheet) Len() int              { return len(me.versions) }
func (me *Sheet) String() string        { return me.name }
func (me *Sheet) bwThreshold(dt int64) uint8 {
	if dt == 0 {
		dt = me.versions[0].dateTimeUnixNano
	}
	return me.parentChapter.bwThreshold(dt)
}

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

	PxCm               float64
	BwThreshold        uint8     `json:",omitempty"`
	FontFactor         float64   `json:",omitempty"`
	GrayDistr          []int     `json:",omitempty"`
	ColDarkestLightest []uint8   `json:",omitempty"`
	PanelsTree         *ImgPanel `json:",omitempty"`
	HomePic            string    `json:",omitempty"`
}

func (me *SheetVerData) PicDirPath(qualiSizeHint int) string {
	return filepath.Join(me.dirPath, "__panels__"+itoa(int(me.parentSheetVer.bwThreshold()))+"_"+ftoa(App.Proj.Sheets.Panel.BorderCm, -1)+"_"+itoa(qualiSizeHint))
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

func (me *SheetVer) bwThreshold() uint8 {
	if me.data.BwThreshold != 0 {
		return me.data.BwThreshold
	}
	return me.parentSheet.bwThreshold(me.dateTimeUnixNano)
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
		me.data = &SheetVerData{parentSheetVer: me, PxCm: dpi1200} //1200dpi
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
	me.data.dirPath = ".ccache/" + svCacheDirNamePrefix + me.id
	me.data.bwFilePath = filepath.Join(me.data.dirPath, "bw."+itoa(int(me.bwThreshold()))+".png")
	me.data.bwSmallFilePath = filepath.Join(me.data.dirPath, "bwsmall."+itoa(int(me.bwThreshold()))+"."+itoa(int(App.Proj.Sheets.Bw.SmallWidth))+".png")
	mkDir(me.data.dirPath)

	// the major prep steps
	didgraydistr := me.ensureGrayDistr(forceFullRedo || len(me.data.GrayDistr) == 0)
	didbw, didbwsmall := me.ensureBwSheetPngs(forceFullRedo)
	didpanels := me.ensurePanelsTree(me.data.PanelsTree == nil || forceFullRedo || didbw)
	didpanelpics := me.ensurePanelPics(forceFullRedo || didpanels)
	didhomepic := me.ensureHomePic(forceFullRedo || didbw || didbwsmall || didpanels)
	didstrips := me.parentSheet.parentChapter.isStrip && me.ensureStrips(forceFullRedo || didbw || didpanels || didpanelpics)

	if shouldsaveprojdata = shouldsaveprojdata || didgraydistr || didpanels || didhomepic || didstrips; shouldsaveprojdata {
		App.Proj.save(false)
	}
	if didWork = shouldsaveprojdata || didbw || didbwsmall || didpanelpics || didstrips; didWork {
		runtime.GC()
	}

	return
}

func (me *SheetVer) ensureBwSheetPngs(force bool) (didBw bool, didBwSmall bool) {
	var exist1, exist2 bool
	for fname, boolptr := range map[string]*bool{me.data.bwFilePath: &exist1, me.data.bwSmallFilePath: &exist2} {
		*boolptr = (fileStat(fname) != nil)
	}

	if didBwSmall = force || !(exist1 && exist2); didBwSmall {
		if didBw = force || !exist1; didBw {
			rmDir(me.data.dirPath) // because BwThreshold might have been changed and..
			mkDir(me.data.dirPath) // ..thus everything in this dir needs re-gen'ing
			if file, err := os.Open(me.fileName); err != nil {
				panic(err)
			} else {
				fileWrite(me.data.bwFilePath, imgToMonochromePng(file, file.Close, me.bwThreshold()))
			}
		}
		if file, err := os.Open(me.data.bwFilePath); err != nil {
			panic(err)
		} else if data := imgDownsizedPng(file, file.Close, int(App.Proj.Sheets.Bw.SmallWidth), true); data != nil {
			fileWrite(me.data.bwSmallFilePath, data)
		} else if err = os.Symlink(filepath.Base(me.data.bwFilePath), me.data.bwSmallFilePath); err != nil {
			panic(err)
		}
	}

	if symlinkpath := filepath.Join(filepath.Dir(me.fileName), "bw."+filepath.Base(me.fileName)); didBw || fileStat(symlinkpath) == nil {
		_ = os.Remove(symlinkpath)
		fileLink(me.data.bwFilePath, symlinkpath)
	}
	return
}

func (me *SheetVer) ensurePanelPics(force bool) bool {
	numpanels, _ := me.panelCount()
	diritems, err := os.ReadDir(me.data.dirPath)
	bgsrcpath := strings.TrimSuffix(me.fileName, ".png") + ".svg"
	bgsrcfile := fileStat(bgsrcpath)
	for _, direntry := range diritems {
		if fileinfo, _ := direntry.Info(); (!direntry.IsDir()) && strings.HasPrefix(direntry.Name(), "bg") &&
			(strings.HasSuffix(direntry.Name(), ".png") || strings.HasSuffix(direntry.Name(), ".svg")) &&
			(bgsrcfile == nil || fileinfo == nil || os.Getenv("REDO_BGS") != "" ||
				bgsrcfile.ModTime().UnixNano() > fileinfo.ModTime().UnixNano() ||
				!strings.HasSuffix(direntry.Name(), ".png")) {
			_ = os.Remove(filepath.Join(me.data.dirPath, direntry.Name()))
		}
	}
	if bgsrcfile != nil {
		pidx, bgsvgsrc := 0, string(fileRead(bgsrcpath))
		me.data.hasBgCol = true
		me.data.PanelsTree.each(func(p *ImgPanel) {
			gid, dstfilepath := "pnl"+itoa(pidx), filepath.Join(me.data.dirPath, "bg"+itoa(pidx)+".png")
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
					srcwidth := App.Proj.Sheets.Bw.SmallWidth
					if idx := strings.Index(bgsvgsrc, `width="`); idx > 0 {
						strw := bgsvgsrc[idx+len(`width="`):]
						strw = strw[:strings.IndexByte(strw, '"')]
						if sw, _ := strconv.ParseUint(strw, 10, 16); sw > 0 {
							srcwidth = uint16(sw)
						} else {
							panic(strw)
						}
					}
					scale := float64(srcwidth) / float64(me.data.PanelsTree.Rect.Max.X)
					pw, ph := int(float64(p.Rect.Dx())*scale), int(float64(p.Rect.Dy())*scale)
					s = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>
						<svg width="` + itoa(pw) + `" height="` + itoa(ph) + `" viewbox="0 0 ` + itoa(pw) + ` ` + itoa(ph) + `" xmlns="http://www.w3.org/2000/svg">` +
						sIf(App.Proj.Sheets.Panel.BgBlur == 0, "",
							`<filter id="leblur"><feGaussianBlur in="SourceGraphic" stdDeviation="`+itoa(App.Proj.Sheets.Panel.BgBlur)+`" /></filter>
							<style type="text/css">path { filter: url(#leblur); }</style>`) +
						s + "</svg>"

					tmpfilepath := "/dev/shm/" + me.id + "_bg" + itoa(pidx) + ".svg"
					fileWrite(tmpfilepath, []byte(s))
					out, errprog := exec.Command("magick", tmpfilepath, "-resize", itoa(int(100.0*App.Proj.Sheets.Panel.BgScale))+"%", dstfilepath).CombinedOutput()
					_ = os.Remove(tmpfilepath)
					if s := trim(string(out)); errprog != nil {
						_ = os.Remove(dstfilepath)
						panic(errprog.Error() + ">>>>" + s + "<<<<")
					} else if len(s) != 0 {
						_ = os.Remove(dstfilepath)
						panic(s)
					}
				}
			}
			pidx++
		})
	}

	if err != nil {
		panic(err)
	}
	forceQualis := map[int]bool{}
	for qidx, quali := range App.Proj.Qualis {
		forceQualis[qidx] = force || (nil == dirStat(me.data.PicDirPath(quali.SizeHint)))
	}
	for qidx := 0; qidx < len(App.Proj.Qualis) && !forceQualis[qidx]; qidx++ {
		quali := App.Proj.Qualis[qidx]
		for pidx, pngdir := 0, me.data.PicDirPath(quali.SizeHint); pidx < numpanels && !forceQualis[qidx]; pidx++ {
			forceQualis[qidx] = bIf(quali.SizeHint == 0,
				nil == fileStat(filepath.Join(me.data.PicDirPath(0), itoa(pidx)+".svg")),
				nil == fileStat(filepath.Join(pngdir, itoa(pidx)+".png")))
		}
	}
	for _, fileinfo := range diritems {
		if rm, name := force, fileinfo.Name(); fileinfo.IsDir() && strings.HasPrefix(name, "__panels__") {
			if got, qstr := -1, name[strings.LastIndexByte(name, '_')+1:]; (!rm) && qstr != "" {
				if q, errparse := strconv.ParseUint(qstr, 10, 64); errparse == nil {
					for i, quali := range App.Proj.Qualis {
						if quali.SizeHint == int(q) {
							got, rm = i, rm || forceQualis[i]
							break
						}
					}
					rm = rm || (got == -1)
				}
			}
			if rm {
				rmDir(filepath.Join(me.data.dirPath, name))
			}
		}
	}
	if forceSome := force; !forceSome {
		for _, f := range forceQualis {
			if forceSome = f; forceSome {
				break
			}
		}
		if !forceSome {
			return false
		}
	}

	for qidx, quali := range App.Proj.Qualis {
		if forceQualis[qidx] {
			mkDir(me.data.PicDirPath(quali.SizeHint))
		}
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
	me.data.PanelsTree.each(func(panel *ImgPanel) {
		work.Add(1)
		go func(pidx int) {
			defer work.Done()
			pw, ph, sw := panel.Rect.Dx(), panel.Rect.Dy(), me.data.PanelsTree.Rect.Dx()
			for qidx, quali := range App.Proj.Qualis {
				if quali.SizeHint == 0 || !forceQualis[qidx] {
					continue
				}
				width := float64(quali.SizeHint) / (float64(sw) / float64(pw))
				height := width / (float64(pw) / float64(ph))
				w, h := int(width), int(height)
				px1cm := me.data.PxCm / (float64(sw) / float64(quali.SizeHint))
				var wassamesize bool
				pngdata := imgSubRectPng(imgsrc.(*image.Gray), panel.Rect, &w, &h, int(px1cm*App.Proj.Sheets.Panel.BorderCm), true, &wassamesize)
				fileWrite(filepath.Join(me.data.PicDirPath(quali.SizeHint), itoa(pidx)+".png"), pngdata)
				if wassamesize {
					break
				}
			}
			if App.Proj.hasSvgQuali() {
				fileWrite(filepath.Join(me.data.PicDirPath(0), itoa(pidx)+".svg"),
					imgSubRectSvg(imgsrc.(*image.Gray), panel.Rect, int(me.data.PxCm*App.Proj.Sheets.Panel.BorderCm)))
			}
		}(pidx)
		pidx++
	})
	work.Wait()
	return true
}

func (me *SheetVer) ensureStrips(force bool) bool {
	if !me.haveAnyTexts("") {
		return false
	}

	const polygonBgCol = "#f7f2eb"
	polygon_bg_col, split := [3]uint8{0xf7, 0xf2, 0xeb}, strings.IndexByte(me.parentSheet.name, '_') > 0

	sheetpngfilepath1, sheetpngfilepath2 := filepath.Join(me.data.dirPath, "strip.1.png"), filepath.Join(me.data.dirPath, "strip.2.png")
	if force = force || fileStat(sheetpngfilepath1) == nil || bIf(split, fileStat(sheetpngfilepath2) == nil, false); !force {
		return false
	}

	sheetsvgfilepath := "/dev/shm/" + filepath.Base(me.fileName) + ".strips.svg"
	sheetpngfilepath := sheetsvgfilepath + ".png"
	var bookGen BookGen
	bookGen.perRow.firstOnly, bookGen.perRow.vertText = !split, me.parentSheet.parentChapter.parentSeries.Name+"@"+strings.ReplaceAll(App.Proj.Site.Host, ".", "·")
	bookGen.genSheetSvg(me, sheetsvgfilepath, false, App.Proj.Langs[0], false, polygonBgCol)
	defer os.Remove(sheetsvgfilepath)
	_ = imgAnyToPng(sheetsvgfilepath, sheetpngfilepath, 0, true, "", 0)
	defer os.Remove(sheetpngfilepath)

	// set polygon_bg_col pixels to fully transparent
	pngsrc := fileRead(sheetpngfilepath)
	img, _, err := image.Decode(bytes.NewReader(pngsrc))
	if err != nil {
		panic(err)
	}
	for x := 0; x < img.Bounds().Dx(); x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			col := img.At(x, y).(color.NRGBA)
			if col.R == polygon_bg_col[0] && col.G == polygon_bg_col[1] && col.B == polygon_bg_col[2] {
				col.A = 0
				img.(draw.Image).Set(x, y, col)
			}
		}
	}

	for _, q := range App.Proj.Qualis {
		if q.StripDefault {
			img = imgDownsized(img, q.SizeHint, true)
			break
		}
	}

	if png_opt := (os.Getenv("NOOPT") == ""); !split {
		fileWrite(sheetpngfilepath1, pngEncode(img))
		if png_opt {
			pngOpt(sheetpngfilepath1)
		}
	} else {
		fileWrite(sheetpngfilepath1, pngEncode(img.(*image.NRGBA).SubImage(
			image.Rect(0, 0, img.Bounds().Max.X, img.Bounds().Max.Y/2))))
		fileWrite(sheetpngfilepath2, pngEncode(img.(*image.NRGBA).SubImage(
			image.Rect(0, img.Bounds().Max.Y/2, img.Bounds().Max.X, img.Bounds().Max.Y))))
		if png_opt {
			pngOpt(sheetpngfilepath1)
			pngOpt(sheetpngfilepath2)
		}
	}
	return true
}

func (me *SheetVer) ensureGrayDistr(force bool) bool {
	if force || len(me.data.GrayDistr) != App.Proj.Sheets.Bw.NumDistrClusters || len(me.data.ColDarkestLightest) != 2 {
		if file, err := os.Open(me.fileName); err != nil {
			panic(err)
		} else {
			me.data.GrayDistr, me.data.ColDarkestLightest = imgGrayDistrs(file, file.Close, App.Proj.Sheets.Bw.NumDistrClusters)
		}
		return true
	}
	return false
}

func (me *SheetVer) ensureHomePic(force bool) (didHomePic bool) {
	if sv, pidx := me.parentSheet.parentChapter.homePic(); sv == me {
		picpath := me.homePicPath(pidx)
		if didHomePic = (force || (picpath != me.data.HomePic) || fileStat(picpath) == nil); didHomePic {
			if me.data.HomePic != "" {
				_ = os.Remove(me.data.HomePic)
			}
			me.data.HomePic = picpath
			fileWrite(picpath,
				imgSubRectPngFile(me.data.bwFilePath, me.panel(pidx).Rect, 0, App.Proj.Site.Gen.HomePicSizeHint, false))
		}
	}
	return
}

func (me *SheetVer) homePicPath(panelIdx int) string {
	return me.data.bwFilePath + ".homepic_" + itoa(panelIdx) + "_" + itoa(App.Proj.Site.Gen.HomePicSizeHint) + ".png"
}

func (me *SheetVer) sizeCm() (float64, float64) {
	return float64(me.data.PanelsTree.Rect.Max.X) / me.data.PxCm, float64(me.data.PanelsTree.Rect.Max.Y) / me.data.PxCm
}

func (me *SheetVer) cmToPx(f float64) int {
	return int(f * me.data.PxCm)
}

func (me *SheetVer) cmsToPxs(fs ...float64) (ret []int) {
	ret = make([]int, len(fs))
	for i, f := range fs {
		ret[i] = me.cmToPx(f)
	}
	return
}

func (me *SheetVer) panel(idx int) (pnl *ImgPanel) {
	pidx := 0
	me.data.PanelsTree.each(func(p *ImgPanel) {
		if pidx == idx {
			pnl = p
		}
		pidx++
	})
	return
}

func (me *SheetVer) panelAt(x int, y int) (pnl *ImgPanel, idx int) {
	pidx := 0
	me.data.PanelsTree.each(func(p *ImgPanel) {
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
	me.data.PanelsTree.each(func(p *ImgPanel) {
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
	detectFromSb := (me.DtStr() > App.Proj.Sheets.Panel.TreeFromStoryboard.After) &&
		(me.parentSheet.parentChapter.storyboardFilePath() != "")
	if did = force || (os.Getenv("FORCE_PTREE") == me.parentSheet.parentChapter.Name) || me.data.PanelsTree == nil ||
		(me.data.PanelsTree.SbBorderOuter != iIf(detectFromSb, App.Proj.Sheets.Panel.TreeFromStoryboard.BorderOuter, 0)) ||
		(me.data.PanelsTree.SbBorderInner != iIf(detectFromSb, App.Proj.Sheets.Panel.TreeFromStoryboard.BorderInner, 0)); did {
		_ = os.Remove(bgtmplsvgfilepath)
		if detectFromSb {
			me.data.PanelsTree = me.parentSheet.parentChapter.panelsTreeFromStoryboard(me)
		} else if file, err := os.Open(me.data.bwFilePath); err != nil {
			panic(err)
		} else {
			me.data.PanelsTree = imgPanelsFile(file, file.Close)
		}
		me.data.PanelsTree.SbBorderOuter = iIf(detectFromSb, App.Proj.Sheets.Panel.TreeFromStoryboard.BorderOuter, 0)
		me.data.PanelsTree.SbBorderInner = iIf(detectFromSb, App.Proj.Sheets.Panel.TreeFromStoryboard.BorderInner, 0)
	} else if os.Getenv("REDO_BGS") != "" {
		_ = os.Remove(bgtmplsvgfilepath)
	}

	scale := float64(App.Proj.Sheets.Bw.SmallWidth) / float64(me.data.PanelsTree.Rect.Max.X)
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
		me.data.PanelsTree.each(func(p *ImgPanel) {
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
	me.data.PanelsTree.each(func(p *ImgPanel) {
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
		me.data.PanelsTree.each(func(p *ImgPanel) {
			numPanels++
		})
	}
	return
}

func (me *SheetVer) homePicName() string {
	if me.data.HomePic != "" {
		return me.parentSheet.parentChapter.parentSeries.Name + "-" + me.parentSheet.parentChapter.Name + ".png"
	}
	return ""
}

func (me *SheetVer) haveAnyTexts(lang string) bool {
	for _, areas := range App.Proj.data.Sv.textRects[me.id] {
		for _, area := range areas {
			for data_lang, text := range area.Data {
				if trim(text) != "" && (lang == "" || data_lang == lang) {
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

func (me *SheetVer) genTextSvgForPanel(panelIdx int, panel *ImgPanel, lang string, forHtml bool, forEbook bool) string {
	panelareas := me.panelAreas(panelIdx)
	if len(panelareas) == 0 {
		return ""
	}

	pw, ph := panel.Rect.Dx(), panel.Rect.Dy()
	s := "<svg viewbox='0 0 " + itoa(pw) + " " + itoa(ph) + "'>"
	for tidx, pta := range panelareas {
		rx, ry, rw, rh := pta.Rect.Min.X-panel.Rect.Min.X, pta.Rect.Min.Y-panel.Rect.Min.Y, pta.Rect.Dx(), pta.Rect.Dy()
		borderandfill := (pta.PointTo != nil)
		if borderandfill {
			rpx, rpy := pta.PointTo.X-panel.Rect.Min.X, pta.PointTo.Y-panel.Rect.Min.Y
			mmh, cmh := int(me.data.PxCm*me.parentSheet.parentChapter.GenPanelSvgText.BoxPolyStrokeWidthCm), int(me.data.PxCm/2.0)
			pl, pr, pt, pb := (rx + mmh), ((rx + rw) - mmh), (ry + mmh), ((ry + rh) - mmh)
			poly := [][2]int{{pl, pt}, {pr, pt}, {pr, pb}, {pl, pb}}
			ins := func(idx int, pts ...[2]int) {
				head, tail := poly[:idx], poly[idx:]
				poly = append(head, append(pts, tail...)...)
			}

			isBalloon := !(pta.PointTo.X == 0 && pta.PointTo.Y == 0)
			if isBalloon {
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
			s += "' class='" + me.parentSheet.parentChapter.GenPanelSvgText.ClsBoxPoly + sIf(isBalloon, " "+me.parentSheet.parentChapter.GenPanelSvgText.ClsBoxPoly+"b", "") + "' stroke-width='" + itoa(mmh) + "px'/>"
		}
		s += "<svg x='" + itoa(rx) + "' y='" + itoa(ry) + "' class='" + sIf(borderandfill, "ptbf", "") + "'>" +
			me.genTextSvgForPanelArea(panelIdx, tidx, &pta, lang, forHtml, forEbook, borderandfill) + "</svg>"
	}

	s += "</svg>"
	return s
}

func (me *SheetVer) genTextSvgForPanelArea(pidx int, tidx int, pta *ImgPanelArea, lang string, forHtml bool, forEbook bool, isBorderAndFill bool) string {
	linex := 0.0
	if pta.PointTo != nil {
		linex = me.data.PxCm * me.parentSheet.parentChapter.GenPanelSvgText.BoxPolyDxCmA4
	}
	fontSizeCmA4, perLineDyCmA4 := me.parentSheet.parentChapter.GenPanelSvgText.FontSizeCmA4, me.parentSheet.parentChapter.GenPanelSvgText.PerLineDyCmA4
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
	if pta.SvgTextTspanStyleAttr == "_storytitle" {
		perLineDyCmA4 *= 1.23
	}
	return me.imgSvgText(pidx, tidx, pta, lang, int(linex), fontSizeCmA4, perLineDyCmA4, forHtml, forEbook, isBorderAndFill)
}

func (me *SheetVerData) pxBounds() (ret image.Rectangle) {
	ret.Min, ret.Max = image.Point{math.MaxInt, math.MaxInt}, image.Point{math.MinInt, math.MinInt}
	me.PanelsTree.each(func(pnl *ImgPanel) {
		if pnl.Rect.Min.X < ret.Min.X {
			ret.Min.X = pnl.Rect.Min.X
		}
		if pnl.Rect.Min.Y < ret.Min.Y {
			ret.Min.Y = pnl.Rect.Min.Y
		}
		if pnl.Rect.Max.X > ret.Max.X {
			ret.Max.X = pnl.Rect.Max.X
		}
		if pnl.Rect.Max.Y > ret.Max.Y {
			ret.Max.Y = pnl.Rect.Max.Y
		}
	})
	return
}

func (me *SheetVer) ensureLetteredPngIfNeeded() {
	if App.Proj.Sheets.GenLetteredPngsInDir == "" {
		return
	}
	dir := filepath.Join(App.Proj.Sheets.GenLetteredPngsInDir, me.parentSheet.parentChapter.parentSeries.Name+"_"+me.parentSheet.parentChapter.Name)
	mkDir(dir)

	fake := BookGen{Sheets: []*SheetVer{me}}
	for _, lang := range App.Proj.Langs {
		if !me.haveAnyTexts(lang) {
			continue
		}
		dst_png_file_path := filepath.Join(dir, me.parentSheet.name+"."+lang+".png")
		if file_info := fileStat(dst_png_file_path); file_info != nil && (!file_info.IsDir()) && file_info.Size() > 0 {
			continue
		}
		fake.genSheetSvgAndPng(me, dst_png_file_path, lang)
	}
}
