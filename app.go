package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

var App struct {
	StaticFilesDirPath string
	Proj               Project
	PrepWork           struct {
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
	App.Proj.load()

	var cmdidx int
	cmdnames := []string{"chromium", "chromium-browser", "chrome", "google-chrome"}
	for i, l := 0, len(cmdnames); i < l; i++ {
		cmdnames = append(cmdnames, cmdnames[i]+"-stable")
	}
	for i, cmdname := range cmdnames {
		if _, nope := exec.LookPath(cmdname); nope == nil {
			cmdidx = i
			break
		}
	}
	browserCmd[0] = cmdnames[cmdidx]
}

func appOnExit() {
	App.Proj.save()
}

var appMainActions = map[string]bool{}
var AppMainActions = A{
	"SiteGen": "Re-generate site fully",
}

func appMainAction(fromGui bool, name string, args map[string]bool) string {
	if appMainActions[name] {
		return "Action '" + name + "' already in progress and not yet done."
	}
	appMainActions[name] = true

	var action func(map[string]bool)
	switch name {
	case "SiteGen":
		action = siteGen
	default:
		return "Unknown action: '" + name + "'"
	}
	if fromGui {
		go func() { defer func() { appMainActions[name] = false }(); action(args) }()
		return "Action '" + name + "' kicked off. Progress printed to stdio."
	} else {
		action(args)
		return ""
	}
}

func appPrepWork() {
	printLn("Starting background pre-processing... (" + strconv.Itoa(len(App.PrepWork.Queue)) + " pending jobs)")
	for true {
		App.PrepWork.Lock()
		if len(App.PrepWork.Queue) == 0 {
			App.PrepWork.Unlock()
			break
		}
		job := App.PrepWork.Queue[0]
		App.PrepWork.Queue = App.PrepWork.Queue[1:]
		App.PrepWork.Unlock()
		job.ensure(false, false)
	}
	printLn("All pending background pre-processings completed.")
}
