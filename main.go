package main

import (
	"os"
	"os/exec"
	"time"
)

func main() {
	appInit()
	if len(os.Args) > 1 {
		appPrepWork()
		args := map[string]bool{}
		for _, arg := range os.Args[2:] {
			args[arg] = true
		}
		if msg := appMainAction(false, os.Args[1], args); msg != "" {
			printLn(msg)
		}
	} else {
		go scanDevicesDetection()
		go httpListenAndServe()
		go launchKioskyBrowser()
		go appPrepWork()
		for canexit := false; !canexit; {
			time.Sleep(time.Second)
			canexit = App.Gui.BrowserClosed && scanJob == nil
			for _, busy := range appMainActions {
				canexit = canexit && !busy
			}
		}
		appOnExit()
	}
}

var browserCmd = []string{"", "--new-window", "--single-process", "--user-data-dir=./.csg/chromium", "--disable-extensions", "--disk-cache-size=128", "--app=http://localhost:4321"}

func launchKioskyBrowser() {
	cmd := exec.Command(browserCmd[0], browserCmd[1:]...)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	if err := cmd.Wait(); err != nil {
		panic(err)
	}
	App.Gui.BrowserClosed = true
}
