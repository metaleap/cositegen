package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

var App struct {
	StaticFilesDirPath string
	Proj               Project
	Gui                struct {
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
	mkDir(".csg")
	mkDir(".csg/meta")
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
	"genfully": "Re-generate site fully (incl. PNGs)",
	"genpages": "Re-generate site (pages only, keep old PNGs)",
}

func appMainAction(fromGui bool, name string, args map[string]bool) string {
	if appMainActions[name] {
		return "Action '" + name + "' already in progress and not yet done."
	}
	appMainActions[name] = true

	var action func(map[string]bool)
	switch name {
	case "genfully":
		action = siteGenFully
	case "genpages":
		action = siteGenPagesOnly
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
	printLn("Preprocessing started...")
	for _, series := range App.Proj.Series {
		for _, chapter := range series.Chapters {
			for _, sheet := range chapter.sheets {
				for _, sv := range sheet.versions {
					if !sv.prep.done {
						sv.prep.Lock()
						if sv.prep.done {
							sv.prep.Unlock()
						} else {
							sv.ensurePrep(true, false)
							sv.prep.done = true
							sv.prep.Unlock()
						}
					}
				}
			}
		}
	}
	printLn("Preprocessing done.")
}
