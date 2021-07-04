package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
)

const siteTmplDirName = "site"
const siteTmplFileName = "site.html"

var browserCmd = []string{"", "--new-window", "--single-process", "--user-data-dir=./.cache/.chromium", "--disable-extensions", "--disk-cache-size=128"}

func main() {
	App.StaticFilesDirPath = filepath.Join(os.Getenv("HOME"), "c/go/src/github.com/metaleap/cositegen/_static")
	appDetectBrowser()
	timedLogged("Loading project (comicsite.json  &  texts.json  &  .cache/data.json)...", func() string {
		numsheets := App.Proj.load()
		return "for " + itoa(numsheets) + " sheets"
	})

	if len(os.Args) > 1 {
		appPrepWork(false)
		args := map[string]bool{}
		for _, arg := range os.Args[2:] {
			args[arg] = true
		}
		if msg := appMainAction(false, os.Args[1], args); msg != "" {
			printLn(msg)
		}
		appOnExit()
	} else {
		go appPrepWork(true)
		if App.Gui.BrowserPid = -1; os.Getenv("NOGUI") == "" {
			go scanDevicesDetection()
			go httpListenAndServe()
			go launchGuiInKioskyBrowser()
		}
		for App.Gui.Exiting = false; !App.Gui.Exiting; time.Sleep(time.Second) {
			appbusy := (scanJob != nil) || (scanDevices == nil) ||
				(0 < atomic.LoadInt32(&numBusyRequests)) || !App.Proj.allPrepsDone
			for _, busy := range appMainActions {
				appbusy = appbusy || busy
			}
			App.Gui.Exiting = (App.Gui.BrowserPid == 0) && !appbusy
		}
		appOnExit()
	}
}

func launchGuiInKioskyBrowser() {
	cmd := exec.Command(browserCmd[0], append(browserCmd[1:], "--app=http://localhost:4321")...)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	App.Gui.BrowserPid = cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		panic(err)
	}
	App.Gui.BrowserPid = 0
}
