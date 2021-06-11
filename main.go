package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var App struct {
	Proj Project
	Gui  struct {
		Closed bool
		State  struct {
			SelectedSeriesName  string
			SelectedChapterName string
		}
	}
}

func main() {
	appInit()
	go serveGui()
	go openGui()
	for !App.Gui.Closed {
		time.Sleep(time.Second)
	}
}

func appInit() {
	if err := os.Mkdir(".build", os.ModePerm); err != nil && !os.IsExist(err) {
		panic(err)
	}
	App.Proj.Load("cosite.json")
}

func openGui() {
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

func serveGui() {
	if err := (&http.Server{
		Addr:    ":4321",
		Handler: http.HandlerFunc(handleHttpReq),
	}).ListenAndServe(); err != nil {
		panic(err)
	}
}

func handleHttpReq(httpResp http.ResponseWriter, httpReq *http.Request) {
	httpResp.Header().Add("Content-Type", "text/html")
	_, _ = httpResp.Write([]byte("Hello, <b><u>World</u></b><hr/>" + httpReq.RequestURI))
}

func jsonLoad(filename string, intoPtr interface{}) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, intoPtr); err != nil {
		panic(err)
	}
}

func jsonStore(filename string, obj interface{}) {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = ioutil.WriteFile(filename, data, os.ModePerm); err != nil {
		panic(err)
	}
}
