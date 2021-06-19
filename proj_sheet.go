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

type SheetVerMeta struct {
	dirPath         string
	bwFilePath      string
	bwSmallFilePath string

	SrcFilePath string
	PanelsTree  *ImgPanel `json:",omitempty"`
}

type SheetVer struct {
	parent      *Sheet
	name        string
	fileName    string
	meta        *SheetVerMeta
	colorLayers bool
	prep        struct {
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
	oldhash := App.Proj.meta.ContentHashes[me.fileName]
	App.Proj.meta.ContentHashes[me.fileName] = curhash
	if oldhash == curhash {
		me.meta = App.Proj.meta.SheetVer[oldhash]
	} else if oldhash != "" {
		me.meta = nil
		delete(App.Proj.meta.SheetVer, oldhash)
		rmDir(".csg/meta/" + oldhash)
	}
	if me.meta == nil {
		shouldsaveprojmeta = true
		me.meta = &SheetVerMeta{SrcFilePath: me.fileName}
		App.Proj.meta.SheetVer[curhash] = me.meta
	}
	me.meta.dirPath = ".csg/meta/" + curhash
	me.meta.bwFilePath = filepath.Join(me.meta.dirPath, "bw."+itoa(int(App.Proj.BwThreshold))+".png")
	me.meta.bwSmallFilePath = filepath.Join(me.meta.dirPath, "bwsmall."+itoa(int(App.Proj.BwThreshold))+".png")
	mkDir(me.meta.dirPath)

	me.ensureMonochrome(forceFullRedo)
	shouldsaveprojmeta = me.ensurePanels(forceFullRedo) || shouldsaveprojmeta

	if shouldsaveprojmeta {
		App.Proj.save()
	}
}

func (me *SheetVer) ensureMonochrome(force bool) {
	if _, err := os.Stat(me.meta.bwFilePath); err != nil || force {
		if err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		rmDir(me.meta.dirPath) // because threshold might have changed...
		mkDir(me.meta.dirPath) // ... and thus the name part of bwFilePath
		if file, err := os.Open(me.fileName); err != nil {
			panic(err)
		} else if data := imgToMonochrome(file, file.Close, uint8(App.Proj.BwThreshold)); data != nil {
			writeFile(me.meta.bwFilePath, data)
		} else if err = os.Symlink("../../../"+me.fileName, me.meta.bwFilePath); err != nil {
			panic(err)
		}
	}
	if _, err := os.Stat(me.meta.bwSmallFilePath); err != nil || force {
		if err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		_ = os.Remove(me.meta.bwSmallFilePath) // sounds unnecessary but.. symlinks
		if file, err := os.Open(me.meta.bwFilePath); err != nil {
			panic(err)
		} else if data := imgDownsized(file, file.Close, 2048); data != nil {
			writeFile(me.meta.bwSmallFilePath, data)
		} else if err = os.Symlink(filepath.Base(me.meta.bwFilePath), me.meta.bwSmallFilePath); err != nil {
			panic(err)
		}
	}
}

func (me *SheetVer) ensurePanels(force bool) bool {
	if old := me.meta.PanelsTree; old == nil || force {
		if file, err := os.Open(me.meta.bwFilePath); err != nil {
			panic(err)
		} else {
			imgpanel := imgPanels(file, file.Close)
			if me.meta.PanelsTree = &imgpanel; old != nil {
				me.meta.PanelsTree.salvageAreasFrom(old)
			}
			return true
		}
	}
	return false
}
