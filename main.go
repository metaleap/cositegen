package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var browserCmd = []string{"", "--new-window", "--single-process", "--user-data-dir=./.csg/chromium", "--disable-extensions", "--disk-cache-size=128"}

func main() {
	App.StaticFilesDirPath = filepath.Join(os.Getenv("HOME"), "c/go/src/github.com/metaleap/cositegen/_static")
	appDetectBrowser()
	App.Proj.load()
	if len(os.Args) > 1 {
		appPrepWork()
		args := map[string]bool{}
		for _, arg := range os.Args[2:] {
			args[arg] = true
		}
		if msg := appMainAction(false, os.Args[1], args); msg != "" {
			printLn(msg)
		}
		appOnExit(false)
	} else {
		go scanDevicesDetection()
		go httpListenAndServe()
		go appPrepWork()
		go launchGuiInKioskyBrowser()
		for canexit := false; !canexit; time.Sleep(time.Second) {
			canexit = (App.Gui.BrowserPid == 0) && !appIsBusy()
		}
		appOnExit(true)
	}
}

func launchGuiInKioskyBrowser() {
	App.Gui.BrowserPid = -1
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
