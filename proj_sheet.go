package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type Sheet struct {
	name     string
	versions []*SheetVer
}

func (me *Sheet) At(i int) fmt.Stringer { return me.versions[i] }
func (me *Sheet) Len() int              { return len(me.versions) }
func (me *Sheet) String() string        { return me.name }

type SheetVerData struct {
	dirPath         string
	bwFilePath      string
	bwSmallFilePath string

	GrayDistr  []int     `json:",omitempty"`
	PanelsTree *ImgPanel `json:",omitempty"`
}

type SheetVer struct {
	parent   *Sheet
	name     string
	fileName string
	data     *SheetVerData
	prep     struct {
		sync.Mutex
		done bool
	}
}

func (me *SheetVer) String() string { return me.fileName }

func (me *SheetVer) ensurePrep(fromBgPrep bool, forceFullRedo bool) {
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
	data, err := os.ReadFile(me.fileName)
	if err != nil {
		panic(err)
	}

	curhash := ""
	for _, b := range contentHash(data) {
		curhash += strconv.FormatUint(uint64(b), 36)
	}
	oldhash := App.Proj.data.ContentHashes[me.fileName]
	App.Proj.data.ContentHashes[me.fileName] = curhash
	if oldhash == curhash {
		me.data = App.Proj.data.SheetVer[oldhash]
	} else if oldhash != "" {
		me.data = nil
		delete(App.Proj.data.SheetVer, oldhash)
		rmDir(".csg/projdata/" + oldhash)
	}
	if me.data == nil {
		shouldsaveprojmeta = true
		me.data = &SheetVerData{}
		App.Proj.data.SheetVer[curhash] = me.data
	}
	me.data.dirPath = ".csg/projdata/" + curhash
	me.data.bwFilePath = filepath.Join(me.data.dirPath, "bw."+itoa(int(App.Proj.BwThreshold))+".png")
	me.data.bwSmallFilePath = filepath.Join(me.data.dirPath, "bwsmall."+itoa(int(App.Proj.BwThreshold))+"."+itoa(int(App.Proj.BwSmallWidth))+".png")
	mkDir(me.data.dirPath)

	me.ensureMonochrome(forceFullRedo)
	shouldsaveprojmeta = me.ensurePanels(forceFullRedo) || shouldsaveprojmeta
	shouldsaveprojmeta = me.ensureColorDistr(forceFullRedo) || shouldsaveprojmeta

	if shouldsaveprojmeta {
		App.Proj.save()
	}
}

func (me *SheetVer) ensureMonochrome(force bool) {
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
	}
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
	if all := App.Proj.data.sheetVerPanelAreas[me.fileName]; len(all) > panelIdx {
		return all[panelIdx]
	}
	return nil
}

func (me *SheetVer) grayDistrs() (r [][3]float64) {
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
