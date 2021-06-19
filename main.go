package main

import (
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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

var browserCmd = []string{"", "--new-window", "--single-process", "--user-data-dir=./.csg/gui", "--disable-extensions", "--disk-cache-size=128", "--app=http://localhost:4321"}

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

func httpListenAndServe() {
	if err := (&http.Server{
		Addr:    ":4321",
		Handler: http.HandlerFunc(httpHandle),
	}).ListenAndServe(); err != nil {
		panic(err)
	}
}

func httpHandle(httpResp http.ResponseWriter, httpReq *http.Request) {
	if ext := path.Ext(httpReq.URL.Path); ext == ".css" || ext == ".js" {
		http.ServeFile(httpResp, httpReq, filepath.Join(App.StaticFilesDirPath, httpReq.URL.Path))
	} else if ext == ".png" {
		http.ServeFile(httpResp, httpReq, filepath.Join(".", httpReq.URL.Path)) //looks redudant but isnt!
	} else if ext != "" {
		http.ServeFile(httpResp, httpReq, filepath.Join("sitetmpl", httpReq.URL.Path))
	} else {
		httpResp.Header().Add("Content-Type", "text/html")
		var notice string
		if action := httpReq.FormValue("main_action"); action != "" {
			if notice = appMainAction(true, action, nil); notice == "" {
				notice = "Action '" + action + "' completed successfully."
			}
		}
		_, _ = httpResp.Write(guiMain(httpReq, notice))
	}
}
