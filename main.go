package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"
)

type Any = interface{}

var App struct {
	StaticFilesDirPath string
	Proj               Project
	Gui                struct {
		Closed bool
		State  struct {
			SelectedSeries  *Series
			SelectedChapter *Chapter
			SelectedScan    *Scan
		}
	}
}

func main() {
	appInit()
	go httpListenAndServe()
	go launchGui()
	for !App.Gui.Closed {
		time.Sleep(time.Second)
	}
}

func appInit() {
	App.StaticFilesDirPath = filepath.Join(os.Getenv("HOME"), "c/go/src/github.com/metaleap/cositegen/_static")
	if err := os.Mkdir(".build", os.ModePerm); err != nil && !os.IsExist(err) {
		panic(err)
	}
	App.Proj.Load("cosite.json")
}

func launchGui() {
	if err := os.Mkdir(".csgtmp", os.ModePerm); err != nil && !os.IsExist(err) {
		panic(err)
	}
	cmd := exec.Command("chromium", "--new-window", "--single-process", "--user-data-dir=./.csgtmp", "--disable-extensions", "--app=http://localhost:4321")
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	if err := cmd.Wait(); err != nil { //&& err.Error() != "signal: segmentation fault" {
		panic(err)
	}
	App.Gui.Closed = true
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
	default:
		_, _ = httpResp.Write(guiMain(httpReq))
	}
}

func jsonLoad(filename string, intoPtr Any) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, intoPtr); err != nil {
		panic(err)
	}
}

func jsonStore(filename string, obj Any) {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = ioutil.WriteFile(filename, data, os.ModePerm); err != nil {
		panic(err)
	}
}
