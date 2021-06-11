package main

import (
	"net/http"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

func main() {
	appInit()
	go httpListenAndServe()
	go launchKioskyBrowser()
	for !App.Gui.BrowserClosed {
		time.Sleep(time.Second)
	}
	appOnExit()
}

func launchKioskyBrowser() {
	cmd := exec.Command("chromium", "--new-window", "--single-process", "--user-data-dir=./.csg_gui", "--disable-extensions", "--app=http://localhost:4321")
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
	httpResp.Header().Add("Content-Type", "text/html")
	switch path.Ext(httpReq.URL.Path) {
	case ".css":
		http.ServeFile(httpResp, httpReq, filepath.Join(App.StaticFilesDirPath, httpReq.URL.Path))
	case ".png":
		http.ServeFile(httpResp, httpReq, filepath.Join(".", httpReq.URL.Path))
	default:
		var notice string
		if action := httpReq.FormValue("main_action"); action != "" {
			if notice = appMainAction(action); notice == "" {
				notice = "Action '" + action + "' completed successfully."
			}
		}
		_, _ = httpResp.Write(guiMain(httpReq, notice))
	}
}
