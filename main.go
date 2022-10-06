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

var browserCmd = []string{"", // filled in by appDetectBrowser()
	"--new-window", "--single-process",
	"--user-data-dir=./.ccache/.chromium",
	"--disable-extensions",
	"--allow-file-access-from-files",

	"--disable-client-side-phishing-detection",
	"--disable-component-extensions-with-background-pages",
	"--disable-default-apps",
	"--mute-audio",
	"--no-default-browser-check",
	"--no-first-run",
	"--use-fake-device-for-media-stream",
	"--allow-running-insecure-content",
	"--autoplay-policy=user-gesture-required",
	"--disable-background-timer-throttling",
	"--disable-ipc-flooding-protection",
	"--disable-notifications",
	"--disable-popup-blocking",
	"--disable-prompt-on-repost",
	"--disable-device-discovery-notifications",
	"--password-store=basic",
	"--disable-background-networking",
	"--disable-background-networking",
	"--disable-breakpad",
	"--disable-component-update",
	"--disable-domain-reliability",
	"--disable-sync",
	"--disable-features=OptimizationHints",
	"--disable-features=Translate",
	"--enable-automation",
	"--deny-permission-prompts",
}

func main() {
	if err := os.Setenv("FASTZOP", "1"); err != nil {
		panic(err)
	}
	if err := os.Setenv("ZOPFAST", "1"); err != nil {
		panic(err)
	}
	App.StaticFilesDirPath = filepath.Join(os.Getenv("GOPATH"), "src/github.com/metaleap/cositegen/_static")
	appDetectBrowser()
	timedLogged("Loading project (cx.json  &  txt.json  &  .ccache/data.json)...", func() string {
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
		App.Gui.BrowserPid = -1
		go appPrepWork(true)
		if os.Getenv("NOGUI") == "" {
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
