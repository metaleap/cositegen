package main

import (
	"fmt"
	"io/ioutil"
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
	dirPath string

	Panels *ImgPanel `json:",omitempty"`
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

	data, err := ioutil.ReadFile(me.fileName)
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
		me.meta = &SheetVerMeta{}
		App.Proj.meta.SheetVer[curhash] = me.meta
	}
	me.meta.dirPath = filepath.Join(".csg_meta", curhash)
	mkDir(me.meta.dirPath)

	me.ensureMonochrome()
	shouldsaveprojmeta = me.ensurePanels() || shouldsaveprojmeta

	if shouldsaveprojmeta {
		App.Proj.save()
	}
}

func (me *SheetVer) ensureMonochrome() {
	bwfilepath := filepath.Join(me.meta.dirPath, "bw.png")
	if _, err := os.Stat(bwfilepath); err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		if file, err := os.Open(me.fileName); err != nil {
			panic(err)
		} else {
			data := imgToMonochrome(file, file.Close, 128)
			writeFile(bwfilepath, data)
		}
	}
}

func (me *SheetVer) ensurePanels() bool {
	if me.meta.Panels == nil {
		bwfilepath := filepath.Join(me.meta.dirPath, "bw.png")
		if file, err := os.Open(bwfilepath); err != nil {
			panic(err)
		} else {
			imgpanel := imgPanels(file, file.Close)
			me.meta.Panels = &imgpanel
			return true
		}
	}
	return false
}
