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
	App.Gui.State.Sel.Chapter = nil
	App.Gui.State.Sel.Sheet = nil
	App.Gui.State.Sel.Series = nil
	App.Proj.load()
}

func appOnExit() {
	App.Proj.save()
}

var appMainActions = map[string]bool{}

func appMainAction(name string, arg string) string {
	if appMainActions[name] {
		return "Action '" + name + "' already in progress and not yet done."
	}
	appMainActions[name] = true
	switch name {
	case "SiteGen":
		go func() { defer func() { appMainActions[name] = false }(); siteGen() }()
	default:
		return "Unknown action: '" + name + "'"
	}
	return "Action '" + name + "' kicked off. Progress printed to stdio."
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
