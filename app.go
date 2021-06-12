package main

import (
	"os"
	"path/filepath"
	"sync"
)

var App struct {
	StaticFilesDirPath string
	Proj               Project
	BgWork             struct {
		sync.Mutex
		Queue []*SheetVer
	}
	Gui struct {
		BrowserClosed bool
		State         struct {
			Sel struct {
				Series  *Series
				Chapter *Chapter
				Sheet   *Sheet
				Ver     *SheetVer
			}
		}
	}
}

func appInit() {
	App.StaticFilesDirPath = filepath.Join(os.Getenv("HOME"), "c/go/src/github.com/metaleap/cositegen/_static")
	mkDir(".csg_meta")
	mkDir(".csg_build")
	App.Gui.State.Sel.Chapter = nil
	App.Gui.State.Sel.Sheet = nil
	App.Gui.State.Sel.Series = nil
	App.Proj.load()
	go appBackgroundWork()
}

func appOnExit() {
	App.Proj.save()
}

func appMainAction(name string) string {
	switch name {
	case "regen_site":
	default:
		return "Unknown action: " + name
	}

	return ""
}

func appBackgroundWork() {
	for true {
		App.BgWork.Lock()
		if len(App.BgWork.Queue) == 0 {
			App.BgWork.Unlock()
			break
		}
		job := App.BgWork.Queue[0]
		App.BgWork.Queue = App.BgWork.Queue[1:]
		App.BgWork.Unlock()
		printLn("Background processing: " + job.fileName + "...")
		job.ensure(false)
	}
	printLn("Background processings complete.")
}
