package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	parent   *Sheet
	name     string
	fileName string
	meta     *SheetVerMeta
}

func (me *SheetVer) String() string { return me.fileName }

func (me *SheetVer) ensure(removeFromWorkQueue bool) {
	var shouldsaveprojmeta bool

	if removeFromWorkQueue {
		App.BgWork.Lock()
		for i, sheetver := range App.BgWork.Queue {
			if me == sheetver {
				App.BgWork.Queue = append(App.BgWork.Queue[:i], App.BgWork.Queue[i+1:]...)
				break
			}
		}
		App.BgWork.Unlock()
	}

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
		if err = os.RemoveAll(filepath.Join(".csg_meta", oldhash)); err != nil && !os.IsNotExist(err) {
			printLn("Failed to rm .csg_meta/" + oldhash + ": " + err.Error())
		}
	}
	if me.meta == nil {
		shouldsaveprojmeta = true
		me.meta = &SheetVerMeta{SrcFilePath: me.fileName}
		App.Proj.meta.SheetVer[curhash] = me.meta
	}
	me.meta.dirPath = filepath.Join(".csg_meta", curhash)
	me.meta.bwFilePath = filepath.Join(me.meta.dirPath, "bw.png")
	me.meta.bwSmallFilePath = filepath.Join(me.meta.dirPath, "bwsmall.png")
	mkDir(me.meta.dirPath)

	me.ensureMonochrome()
	shouldsaveprojmeta = me.ensurePanels() || shouldsaveprojmeta

	if shouldsaveprojmeta {
		App.Proj.save()
	}
}

func (me *SheetVer) ensureMonochrome() {
	if _, err := os.Stat(me.meta.bwFilePath); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		_ = os.Remove(me.meta.bwFilePath) // sounds weird but due to potential rare symlink edge case
		if file, err := os.Open(me.fileName); err != nil {
			panic(err)
		} else if data := imgToMonochrome(file, file.Close, 128); data != nil {
			writeFile(me.meta.bwFilePath, data)
		} else if err = os.Symlink(filepath.Base(me.fileName), me.meta.bwFilePath); err != nil {
			panic(err)
		}
	}
	if _, err := os.Stat(me.meta.bwSmallFilePath); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		_ = os.Remove(me.meta.bwSmallFilePath) // sounds weird but due to potential rare symlink edge case
		if file, err := os.Open(me.meta.bwFilePath); err != nil {
			panic(err)
		} else if data := imgDownsized(file, file.Close, 2048, false); data != nil {
			writeFile(me.meta.bwSmallFilePath, data)
		} else if err = os.Symlink(filepath.Base(me.meta.bwFilePath), me.meta.bwSmallFilePath); err != nil {
			panic(err)
		}
	}
}

func (me *SheetVer) ensurePanels() bool {
	if me.meta.PanelsTree == nil {
		if file, err := os.Open(me.meta.bwFilePath); err != nil {
			panic(err)
		} else {
			imgpanel := imgPanels(file, file.Close)
			me.meta.PanelsTree = &imgpanel
			return true
		}
	}
	return false
}
