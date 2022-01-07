package main

import (
	"errors"
	"image/color"
	_ "image/png"
	"io/fs"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var appMainActions = map[string]bool{}
var AppMainActions = A{
	"sitegen": "Re-generate site",
}

var App struct {
	StaticFilesDirPath string
	Proj               Project
	Gui                struct {
		Exiting    bool
		BrowserPid int
		State      struct {
			Sel struct {
				Series  *Series
				Chapter *Chapter
				Sheet   *Sheet
				Ver     *SheetVer
			}
		}
	}
	pngOptBusy bool
}

func appDetectBrowser() {
	var cmdidx int
	cmdnames := []string{"chromium", "chromium-browser", "chrome", "google-chrome"}
	for i, l := 0, len(cmdnames); i < l; i++ {
		cmdnames = append(cmdnames, cmdnames[i]+"-stable")
	}
	for i, cmdname := range cmdnames {
		if _, nope := exec.LookPath(cmdname); nope == nil {
			cmdidx = i
			break
		}
	}
	browserCmd[0] = cmdnames[cmdidx]
}

func appOnExit() {
	App.Proj.save(false)
}

func appMainAction(fromGui bool, name string, args map[string]struct{}) string {
	if appMainActions[name] {
		return "Action '" + name + "' already in progress and not yet done."
	}
	appMainActions[name] = true

	var action func(map[string]struct{})
	switch name {
	case "gen":
		action = func(flags map[string]struct{}) { siteGen{}.genSite(fromGui, flags) }
	default:
		s := "Unknown action: '" + name + "', try one of these:"
		for name, desc := range AppMainActions {
			s += "\n\t" + name + "\t\t" + desc
		}
		return s
	}

	if fromGui {
		go func() { defer func() { appMainActions[name] = false }(); action(args) }()
		return "Action '" + name + "' kicked off. Progress printed to stdio."
	}
	action(args)
	return ""
}

func appPrepWork(fromGui bool) {
	var didsomework []string
	App.Proj.allPrepsDone = false
	timedLogged("Reprocessing...", func() string {
		var numjobs, numwork int
		for _, series := range App.Proj.Series {
			var didanywork bool
			for _, chapter := range series.Chapters {
				for _, sheet := range chapter.sheets {
					for _, sv := range sheet.versions {
						if !sv.prep.done {
							sv.prep.Lock()
							if !sv.prep.done {
								didwork := sv.ensurePrep(true, false)
								if sv.prep.done, numjobs = true, numjobs+1; didwork {
									numwork, didanywork, didsomework = numwork+1, true, append(didsomework, sv.fileName)
								}
							}
							sv.prep.Unlock()
						}
					}
				}
			}
			var thumbsrcfilenames FilePathsSortingByModTime
			for _, sv := range series.allSheetVersSortedByScanDate() {
				thumbsrcfilenames = append(thumbsrcfilenames, sv.data.bwSmallFilePath)
			}
			const maxthumbs = 22
			if App.Proj.NumSheetsInHomeBgs > maxthumbs {
				App.Proj.NumSheetsInHomeBgs = maxthumbs
			}
			thumbname := siteGen{}.nameThumb(series)
			thumbfilepath, idxdot := ".ccache/"+thumbname+".png", 1+strings.LastIndexByte(thumbname, '.')
			for i := 0; i < maxthumbs; i++ {
				if i != App.Proj.NumSheetsInHomeBgs {
					_ = os.Remove(".ccache/" + thumbname[:idxdot] + itoa(i) + ".png")
				}
			}
			if didanywork || len(thumbsrcfilenames) == 0 || App.Proj.NumSheetsInHomeBgs == 0 {
				_ = os.Remove(thumbfilepath)
			}
			if len(thumbsrcfilenames) > 0 && App.Proj.NumSheetsInHomeBgs > 0 &&
				(didanywork || nil == fileStat(thumbfilepath)) {
				if len(thumbsrcfilenames) > App.Proj.NumSheetsInHomeBgs {
					thumbsrcfilenames = thumbsrcfilenames[len(thumbsrcfilenames)-App.Proj.NumSheetsInHomeBgs:]
				}
				rand.Shuffle(len(thumbsrcfilenames), func(i int, j int) {
					thumbsrcfilenames[i], thumbsrcfilenames[j] = thumbsrcfilenames[j], thumbsrcfilenames[i]
				})
				fileWrite(thumbfilepath, imgStitchHorizontally(thumbsrcfilenames, 320, 44, color.NRGBA{0, 0, 0, 0}))
			}
		}
		App.Proj.allPrepsDone = true
		return "for " + itoa(numwork) + "/" + itoa(numjobs) + " reprocessing jobs: \n\t\t" + strings.Join(didsomework, "\n\t\t") + "\n"
	})
	if fromGui {
		pngOptsLoop()
	}
}

func pngOptsLoop() {
	App.pngOptBusy = true
	defer func() { App.pngOptBusy = false }()

	for dirfs := os.DirFS("."); !App.Gui.Exiting; time.Sleep(15 * time.Minute) {
		dels := false
		for k := range App.Proj.data.PngOpt {
			if fileStat(k) == nil {
				delete(App.Proj.data.PngOpt, k)
				dels = true
			}
		}
		if dels {
			App.Proj.save(false)
		}
		if App.Gui.Exiting {
			return
		}

		numdone, matches, totalsize, errexiting := 0, FilePathsSortingByFileSize{}, uint64(0), errors.New("exiting")
		if err := fs.WalkDir(dirfs, ".", func(fspath string, dir fs.DirEntry, err error) error {
			if App.Gui.Exiting {
				return errexiting
			}
			if fileinfo, err := os.Lstat(fspath); err == nil && (!fileinfo.IsDir()) &&
				(!fileIsSymlink(fileinfo)) && strings.HasSuffix(fspath, ".png") &&
				!(strings.HasPrefix(fspath, ".build/") || strings.HasPrefix(fspath, ".chromium/") || strings.Contains(fspath, "/.pngtmp/sh.")) {
				matches, totalsize = append(matches, fspath), totalsize+uint64(fileinfo.Size())
			}
			return nil
		}); err == errexiting {
			return
		} else if err != nil {
			printLn("PNGOPT Walk: " + err.Error())
		}
		sort.Sort(matches)

		var numfullynew int
		for i := 0; i < len(matches); i++ {
			pngfilepath := matches[i]
			if _, known := App.Proj.data.PngOpt[pngfilepath]; !known {
				numfullynew++
			}
		}
		printLn("PNGOPT: found", len(matches), "("+itoa(numfullynew)+" new) PNGs (~"+itoa(int(totalsize/(1024*1024)))+"MB) to scrutinize...")
		for _, pngfilename := range matches {
			if App.Gui.Exiting {
				return
			}
			if pngOpt(pngfilename) {
				numdone++
				App.Proj.save(false)
			}
		}
		printLn("PNGOPT:", len(matches), "scrutinized &", numdone, "processed, taking a quarter-hour nap...")
	}
}

func pngOpt(pngFilePath string) bool {
	curfiledata := fileRead(pngFilePath)
	curfilehash := string(contentHashStr(curfiledata))
	lastopt, skip := App.Proj.data.PngOpt[pngFilePath]
	if skip = skip && (lastopt[1] == itoa(len(curfiledata))) &&
		(lastopt[2] == curfilehash); skip {
		return false
	}

	cmd := exec.Command("pngbattle", pngFilePath)
	if strings.Contains(pngFilePath, "/bg") {
		cmd.Env = append(os.Environ(), "NO_RGBA_CHECK=1")
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	go cmd.Wait()
	for ; cmd.ProcessState == nil; time.Sleep(time.Second) {
		if App.Gui.Exiting {
			_ = cmd.Process.Kill()
			_ = exec.Command("killall", "zopflipng").Run()
			_ = exec.Command("killall", "pngbattle").Run()
			return false
		}
	}
	if !cmd.ProcessState.Success() {
		printLn(cmd.ProcessState.String())
		return false
	}
	if filedata, err := os.ReadFile(pngFilePath); err == nil {
		newfilehash := string(contentHashStr(filedata))
		known_byid, known_bymeta := App.Proj.data.Sv.ById[curfilehash] != nil, App.Proj.data.Sv.IdsToFileMeta[curfilehash].FilePath != ""
		_, known_bytexts := App.Proj.data.Sv.textRects[curfilehash]
		crashit := (newfilehash != curfilehash) && (known_byid || known_bymeta || known_bytexts)
		if crashit {
			go exec.Command("beepintime", "1ns").Run()
			if known_byid {
				App.Proj.data.Sv.ById[newfilehash] = App.Proj.data.Sv.ById[curfilehash]
				delete(App.Proj.data.Sv.ById, curfilehash)
			}
			if known_bymeta {
				App.Proj.data.Sv.IdsToFileMeta[newfilehash] = App.Proj.data.Sv.IdsToFileMeta[curfilehash]
				delete(App.Proj.data.Sv.IdsToFileMeta, curfilehash)
				App.Proj.data.Sv.fileNamesToIds[App.Proj.data.Sv.IdsToFileMeta[newfilehash].FilePath] = newfilehash
			}
			if known_bytexts {
				App.Proj.data.Sv.textRects[newfilehash] = App.Proj.data.Sv.textRects[curfilehash]
				delete(App.Proj.data.Sv.textRects, curfilehash)
			}
			if err := os.Rename(".ccache/"+svCacheDirNamePrefix+curfilehash, ".ccache/"+svCacheDirNamePrefix+newfilehash); err != nil {
				printLn("MUST mv manually:", curfilehash, "to", newfilehash, "because:", err.Error())
			}
		}
		App.Proj.data.PngOpt[pngFilePath] = []string{
			itoa(len(curfiledata)),
			itoa(len(filedata)),
			newfilehash,
		}
		if crashit {
			for k, v := range App.Proj.data.PngOpt {
				if pref := ".ccache/" + svCacheDirNamePrefix + curfilehash + "/"; strings.HasPrefix(k, pref) {
					App.Proj.data.PngOpt[".ccache/"+svCacheDirNamePrefix+newfilehash+"/"+k[len(pref):]] = v
					delete(App.Proj.data.PngOpt, k)
				}
			}
			if svgfilepath := ".ccache/" + svCacheDirNamePrefix + newfilehash + "/" + strings.ReplaceAll(filepath.Base(pngFilePath), ".png", ".svg"); fileStat(svgfilepath) != nil {
				if svg := fileRead(svgfilepath); len(svg) != 0 {
					fileWrite(svgfilepath, []byte(strings.ReplaceAll(string(svg), curfilehash, newfilehash)))
				}
			}
			App.Proj.save(os.Getenv("NOGUI") != "")
			panic("relinked hash from " + curfilehash + " to " + newfilehash + " — intentional crash, restart manually")
		} else if strings.HasSuffix(pngFilePath, "/bwsmall."+itoa(int(App.Proj.BwThresholds[0]))+"."+itoa(int(App.Proj.BwSmallWidth))+".png") {
			if hashid := filepath.Base(filepath.Dir(pngFilePath)); App.Proj.data.Sv.ById != nil {
				if svdata := App.Proj.data.Sv.ById[hashid]; svdata != nil && svdata.parentSheetVer != nil {
					_ = os.Remove(filepath.Join(svdata.dirPath, strings.TrimSuffix(
						filepath.Base(svdata.parentSheetVer.fileName), ".png")+".svg"))
				}
			}
		}
		return true
	}
	return false
}
