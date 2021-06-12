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
	versions []*SheetVersion
}

func (me *Sheet) At(i int) fmt.Stringer { return me.versions[i] }
func (me *Sheet) Len() int              { return len(me.versions) }
func (me *Sheet) String() string        { return me.name }

type SheetVersion struct {
	parent   *Sheet
	name     string
	fileName string
	meta     struct {
		contentHash string
	}
}

func (me *SheetVersion) String() string { return me.fileName }

func (me *SheetVersion) ensureFullMeta(removeFromWorkQueue bool) {
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
	me.meta.contentHash = ""
	for _, b := range contentHash(data) {
		me.meta.contentHash += strconv.FormatUint(uint64(b), 36)
	}
	oldhash := App.Proj.meta.ContentHashes[me.fileName]
	if oldhash != "" && oldhash != me.meta.contentHash {
		if err = os.RemoveAll(filepath.Join(".csg_meta", oldhash)); err != nil && !os.IsNotExist(err) {
			printLn("Failed to rm .csg_meta/" + oldhash + ": " + err.Error())
		}
	}
	App.Proj.meta.ContentHashes[me.fileName] = me.meta.contentHash
	metadirpath := filepath.Join(".csg_meta", me.meta.contentHash)
	mkDir(metadirpath)

	{ // ensure monochrome sheet ver
		bwfilepath := filepath.Join(metadirpath, "bw.png")
		if _, err := os.Stat(bwfilepath); err != nil {
			if !os.IsNotExist(err) {
				panic(err)
			}
			if file, err := os.Open(me.fileName); err != nil {
				panic(err)
			} else {
				data := imgToMonochrome(file, 128)
				_ = file.Close()
				writeFile(bwfilepath, data)
			}
		}
	}
}
