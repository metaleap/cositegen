package main

import (
	"bytes"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

var numBusyRequests int32

func httpListenAndServe() {
	if err := (&http.Server{
		Addr:    ":4321",
		Handler: http.HandlerFunc(httpHandle),
	}).ListenAndServe(); err != nil {
		panic(err)
	}
}

func httpHandle(httpResp http.ResponseWriter, httpReq *http.Request) {
	atomic.AddInt32(&numBusyRequests, 1)
	defer func() { atomic.AddInt32(&numBusyRequests, -1) }()

	if ext := path.Ext(httpReq.URL.Path); ext == ".css" || ext == ".js" {
		http.ServeFile(httpResp, httpReq, filepath.Join(App.StaticFilesDirPath, httpReq.URL.Path))
	} else if ext == ".png" || ext == ".svg" {
		http.ServeFile(httpResp, httpReq, filepath.Join("." /*looks redudant but isnt!*/, httpReq.URL.Path))
	} else if ext != "" {
		http.ServeFile(httpResp, httpReq, filepath.Join(siteTmplDirName, httpReq.URL.Path))
	} else if strings.Contains(httpReq.URL.Path, ".png/") {
		httpServeDynPng(httpResp, httpReq)
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

func httpServeDynPng(httpResp http.ResponseWriter, httpReq *http.Request) {
	var pngdata []byte
	tmpfilename := "/dev/shm/" + strings.Replace(httpReq.URL.Path, "/", "_", -1)
	pngdata, _ = os.ReadFile(tmpfilename)

	if len(pngdata) == 0 {
		idx := strings.Index(httpReq.URL.Path, ".png/")
		urlpath, urlargstr := httpReq.URL.Path[:idx+len(".png")], httpReq.URL.Path[idx+len(".png/"):]
		filename := filepath.Join("." /*looks redundant but isnt!*/, urlpath)
		file, err := os.Open(filename)
		if err != nil {
			panic(err)
		}

		args := strings.Split(urlargstr, "/")
		t := App.Proj.BwThresholds[0]
		if qt := args[0]; qt != "" {
			if ui8, err := strconv.ParseUint(qt, 0, 8); err != nil {
				panic(err)
			} else if ui8 > 255 {
				panic(ui8)
			} else {
				t = uint8(ui8)
			}
		}

		w := 0
		pngdata = imgToMonochrome(file, file.Close, t)
		if len(args) > 1 {
			if qw := args[1]; qw != "" {
				if ui, err := strconv.ParseUint(qw, 0, 64); err != nil {
					panic(err)
				} else {
					w = int(ui)
				}
			}
		}
		if w != 0 {
			pngdata = imgDownsized(bytes.NewReader(pngdata), nil, int(w), false)
		}
		_ = os.WriteFile(tmpfilename, pngdata, os.ModePerm)
	}

	httpResp.Header().Add("Cache-Control", "public")
	httpResp.Header().Add("Cache-Control", "max-age=8640000")
	httpResp.Header().Add("Content-Type", "image/png")
	httpResp.Header().Add("Content-Length", strconv.FormatUint(uint64(len(pngdata)), 10))
	_, _ = httpResp.Write(pngdata)
}
