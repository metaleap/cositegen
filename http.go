package main

import (
	"bytes"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

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
		http.ServeFile(httpResp, httpReq, filepath.Join("." /*looks redudant but isnt!*/, httpReq.URL.Path))
	} else if ext != "" {
		http.ServeFile(httpResp, httpReq, filepath.Join("sitetmpl", httpReq.URL.Path))
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
	idx := strings.Index(httpReq.URL.Path, ".png/")
	urlpath, urlargstr := httpReq.URL.Path[:idx+len(".png")], httpReq.URL.Path[idx+len(".png/"):]
	filename := filepath.Join("." /*looks redudant but isnt!*/, urlpath)
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	args := strings.Split(urlargstr, "/")
	t := App.Proj.BwThreshold
	if qt := args[0]; qt != "" {
		if ui8, err := strconv.ParseUint(qt, 0, 8); err != nil {
			panic(err)
		} else if ui8 > 255 {
			panic(ui8)
		} else {
			t = uint8(ui8)
		}
	}

	w, pngdata := 0, imgToMonochrome(file, file.Close, t)
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
		pngdata = imgDownsized(bytes.NewReader(pngdata), nil, int(w))
	}

	httpResp.Header().Add("Cache-Control", "public")
	httpResp.Header().Add("Cache-Control", "max-age=8640000")
	httpResp.Header().Add("Content-Type", "image/png")
	httpResp.Header().Add("Content-Length", strconv.FormatUint(uint64(len(pngdata)), 10))
	bl, _ := httpResp.Write(pngdata)
	printLn(bl, len(pngdata))
}
