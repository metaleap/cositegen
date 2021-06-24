package main

import (
	"os/exec"
	"sync/atomic"
)

var appMainActions = map[string]bool{}
var AppMainActions = A{
	"sitegen": "Re-generate site",
}

var App struct {
	StaticFilesDirPath string
	Proj               Project
	Gui                struct {
		BrowserPid int
		State      struct {
			Sel struct {
				Series  *Series
				Chapter *Chapter
				Sheet   *Sheet
				Ver     *SheetVer
			}
		}
	}
}

func appDetectBrowser() {
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
	rmDir(".csg/tmp")
}

func appIsBusy() (tooBusy bool) {
	tooBusy = (scanJob != nil) || (scanDevices == nil) ||
		(atomic.LoadInt32(&numBusyRequests) > 0) || !App.Proj.allPrepsDone
	for _, busy := range appMainActions {
		tooBusy = tooBusy || busy
	}
	return
}

func appMainAction(fromGui bool, name string, args map[string]bool) string {
	if appMainActions[name] {
		return "Action '" + name + "' already in progress and not yet done."
	}
	appMainActions[name] = true

	var action func(map[string]bool)
	switch name {
	case "sitegen":
		action = siteGen{}.genSite
	default:
		s := "Unknown action: '" + name + "', try one of these:"
		for name, desc := range AppMainActions {
			s += "\n\t" + name + "\t\t" + desc
		}
		return s
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
	App.Proj.allPrepsDone = false
	timedLogged("Preprocessing...", func() string {
		var numjobs, numwork int
		for _, series := range App.Proj.Series {
			for _, chapter := range series.Chapters {
				for _, sheet := range chapter.sheets {
					for _, sv := range sheet.versions {
						if !sv.prep.done {
							sv.prep.Lock()
							if !sv.prep.done {
								if sv.prep.done, numjobs = true, numjobs+1; sv.ensurePrep(true, false) {
									numwork++
								}
							}
							sv.prep.Unlock()
						}
					}
				}
			}
		}
		App.Proj.allPrepsDone = true
		return "for " + itoa(numwork) + "/" + itoa(numjobs) + " preprocessing job(s)"
	})
}
