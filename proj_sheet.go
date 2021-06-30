package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
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

func (me *Sheet) versionNamedOrLatest(verName string) *SheetVer {
	for _, sv := range me.versions {
		if sv.name == verName {
			return sv
		}
	}
	return me.versions[0]
}

type SheetVerData struct {
	parentSheetVer  *SheetVer
	dirPath         string
	bwFilePath      string
	bwSmallFilePath string
	pngDirPath      string

	DateTimeUnixNano int64
	GrayDistr        []int     `json:",omitempty"`
	PanelsTree       *ImgPanel `json:",omitempty"`
}

type SheetVer struct {
	parentSheet *Sheet
	id          string
	name        string
	fileName    string
	data        *SheetVerData
	prep        struct {
		sync.Mutex
		done bool
	}
}

func (me *SheetVer) Px1Cm() float64 {
	return float64(me.data.PanelsTree.Rect.Max.Y-me.data.PanelsTree.Rect.Min.Y) / 21.0
}

func (me *SheetVer) String() string { return me.fileName }

func (me *SheetVer) load() {
	data, err := os.ReadFile(me.fileName)
	if err != nil {
		panic(err)
	}

	me.id = ""
	for _, b := range contentHash(data) {
		me.id += strconv.FormatUint(uint64(b), 36)
	}
	App.Proj.data.Sv.fileNamesToIds[me.fileName] = me.id
	App.Proj.data.Sv.IdsToFileNames[me.id] = me.fileName
	me.data = App.Proj.data.Sv.ById[me.id]
}

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
	shouldsaveprojmeta := forceFullRedo
	if me.data == nil {
		shouldsaveprojmeta = true
		me.data = &SheetVerData{DateTimeUnixNano: time.Now().UnixNano()}
		App.Proj.data.Sv.ById[me.id] = me.data
	}
	me.data.parentSheetVer = me
	me.data.dirPath = ".csg/sv/" + me.id
	me.data.bwFilePath = filepath.Join(me.data.dirPath, "bw."+itoa(int(App.Proj.BwThreshold))+".png")
	me.data.bwSmallFilePath = filepath.Join(me.data.dirPath, "bwsmall."+itoa(int(App.Proj.BwThreshold))+"."+itoa(int(App.Proj.BwSmallWidth))+".png")
	me.data.pngDirPath = "png." + itoa(int(App.Proj.BwThreshold))
	for _, q := range App.Proj.Qualis {
		me.data.pngDirPath += "." + itoa(q.SizeHint)
	}
	me.data.pngDirPath = filepath.Join(me.data.dirPath, me.data.pngDirPath)

	mkDir(me.data.dirPath)

	didWork = me.ensureBwSheetPngs(forceFullRedo)
	shouldsaveprojmeta = me.ensurePanels(forceFullRedo || didWork) || shouldsaveprojmeta
	shouldsaveprojmeta = me.ensureBwPanelPngs(forceFullRedo || didWork) || shouldsaveprojmeta
	shouldsaveprojmeta = me.ensureColorDistr(forceFullRedo || shouldsaveprojmeta) || shouldsaveprojmeta

	if didWork = didWork || shouldsaveprojmeta; shouldsaveprojmeta {
		App.Proj.save()
	}
	return
}

func (me *SheetVer) ensureBwSheetPngs(force bool) bool {
	var exist1, exist2 bool
	for fname, bptr := range map[string]*bool{me.data.bwFilePath: &exist1, me.data.bwSmallFilePath: &exist2} {
		if fileinfo, err := os.Stat(fname); err == nil && !fileinfo.IsDir() {
			*bptr = true
		} else if !os.IsNotExist(err) {
			panic(err)
		}
	}

	if force || !(exist1 && exist2) {
		rmDir(me.data.dirPath) // because BwThreshold or BwSmallWidth might have been..
		mkDir(me.data.dirPath) // ..changed and thus the file names: so rm stale ones.
		if file, err := os.Open(me.fileName); err != nil {
			panic(err)
		} else if data := imgToMonochrome(file, file.Close, uint8(App.Proj.BwThreshold)); data != nil {
			writeFile(me.data.bwFilePath, data)
		} else if err = os.Symlink("../../../"+me.fileName, me.data.bwFilePath); err != nil {
			panic(err)
		}
		if file, err := os.Open(me.data.bwFilePath); err != nil {
			panic(err)
		} else if data := imgDownsized(file, file.Close, int(App.Proj.BwSmallWidth)); data != nil {
			writeFile(me.data.bwSmallFilePath, data)
		} else if err = os.Symlink(filepath.Base(me.data.bwFilePath), me.data.bwSmallFilePath); err != nil {
			panic(err)
		}
		return true
	}
	return false
}

func (me *SheetVer) ensureBwPanelPngs(force bool) bool {
	var numpanels int
	me.data.PanelsTree.iter(func(panel *ImgPanel) {
		numpanels++
	})
	for pidx := 0; pidx < numpanels && !force; pidx++ {
		if fileinfo, err := os.Stat(filepath.Join(me.data.pngDirPath, itoa(pidx)+"."+itoa(App.Proj.Qualis[0].SizeHint)+".png")); err != nil || fileinfo.IsDir() {
			force = true
		}
	}
	if !force {
		return false
	}

	rmDir(me.data.pngDirPath)
	mkDir(me.data.pngDirPath)
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
			for _, quali := range App.Proj.Qualis {
				pw, ph, sw := panel.Rect.Max.X-panel.Rect.Min.X, panel.Rect.Max.Y-panel.Rect.Min.Y, me.data.PanelsTree.Rect.Max.X-me.data.PanelsTree.Rect.Min.X
				width := float64(quali.SizeHint) / (float64(sw) / float64(pw))
				height := width / (float64(pw) / float64(ph))
				w, h := int(width), int(height)
				var wassamesize bool
				pngdata := imgSubRectPng(imgsrc.(*image.Gray), panel.Rect, &w, &h, quali.SizeHint/640, 0, false, &wassamesize)
				writeFile(filepath.Join(me.data.pngDirPath, itoa(pidx)+"."+itoa(quali.SizeHint)+".png"), pngdata)
				if wassamesize {
					break
				}
			}
			work.Done()
		}(pidx)
		pidx++
	})
	work.Wait()

	return true
}

func (me *SheetVer) ensureColorDistr(force bool) bool {
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

func (me *SheetVer) ensurePanels(force bool) bool {
	if force || me.data.PanelsTree == nil {
		if file, err := os.Open(me.data.bwFilePath); err != nil {
			panic(err)
		} else {
			imgpanel := imgPanels(file, file.Close)
			me.data.PanelsTree = &imgpanel
			return true
		}
	}
	return false
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

func (me *SheetVer) percentTranslated() map[string]float64 {
	haveany, ret := false, map[string]float64{}
	for _, areas := range App.Proj.data.Sv.textRects[me.id] {
		for _, area := range areas {
			for langid, text := range area.Data {
				if trim(text) != "" {
					haveany, ret[langid] = true, ret[langid]+1
				}
			}
		}
	}
	if !haveany {
		return nil
	}
	for _, langid := range App.Proj.Langs[1:] {
		ret[langid] = ret[langid] * (100.0 / ret[App.Proj.Langs[0]])
	}
	delete(ret, App.Proj.Langs[0])
	return ret
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
