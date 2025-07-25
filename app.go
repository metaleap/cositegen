package main

import (
	"errors"
	_ "image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var appMainActions = map[string]bool{}
var AppMainActions = A{
	"gen":  "Re-generate site",
	"book": "Generate book",
	"cfg":  "Edit cx.json",
	"pngs": "Generate lettered PNGs",
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

	pngOptBusy  bool
	pngDynServe []string
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
	for _, pngfilepath := range App.pngDynServe {
		_ = os.Remove(pngfilepath)
	}
}

func appMainAction(fromGui bool, name string, args map[string]bool) string {
	if appMainActions[name] {
		return "Action '" + name + "' already in progress and not yet done."
	}
	appMainActions[name] = true

	var action func(map[string]bool)
	switch name {
	case "gen":
		action = func(flags map[string]bool) { siteGen{}.genSite(fromGui, flags) }
	case "book":
		action = makeBook
	case "pngs":
		action = makePngs
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
	App.Proj.allPrepsDone = false
	timedLogged("Reprocessing...", func() string {
		var numjobs, numwork int
		for _, series := range App.Proj.Series {
			for _, chapter := range series.Chapters {
				hp_sv, hp_pidx := chapter.homePic()
				for _, sheet := range chapter.sheets {
					for _, sv := range sheet.versions {
						if !sv.prep.done {
							sv.prep.Lock()
							if !sv.prep.done {
								didwork := sv.ensurePrep(true, false)
								if sv.prep.done, numjobs = true, numjobs+1; didwork {
									printLn(time.Now().Format("15:04:05")+"\t#"+itoa(1+numwork)+"\t"+sv.FileName, "BW:", sv.bwThreshold())
									numwork = numwork + 1
								}
							}
							sv.prep.Unlock()
						}
						if num_panels, _ := sv.panelCount(); num_panels > 0 {
							for pidx := 0; pidx < num_panels; pidx++ {
								hp_path := sv.homePicPath(pidx)
								if fileStat(hp_path) != nil && (sv != hp_sv || pidx != hp_pidx) {
									_ = os.Remove(hp_path)
								}
							}
						}
					}
				}
			}
		}
		App.Proj.allPrepsDone = true
		return "for " + itoa(numwork) + "/" + itoa(numjobs) + " reprocessing jobs"
	})
	if fromGui && os.Getenv("NOOPT") == "" {
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
				!(strings.HasPrefix(fspath, ".build/") || strings.HasPrefix(fspath, ".chromium/") || strings.Contains(fspath, "/.pngtmp/sh.") || strings.Contains(fspath, "/.pngtmp/")) {
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

	if os.Getenv("OPTFORCE") == "" {
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
	}
	if filedata, err := os.ReadFile(pngFilePath); err == nil {
		newfilehash := string(contentHashStr(filedata))
		known_byid, known_bymeta := App.Proj.data.Sv.ById[curfilehash] != nil, App.Proj.data.Sv.IdsToFileMeta[curfilehash].FilePath != ""
		_, known_bytexts := App.Proj.data.Sv.textRects[curfilehash]
		crashit := (newfilehash != curfilehash) &&
			(known_byid || known_bymeta || known_bytexts)
		if crashit {
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
			if msg := "relinked hash from " + curfilehash + " to " + newfilehash; os.Getenv("NOCRASH") == "" {
				_ = exec.Command("beepintime", "1ns").Start()
				App.Proj.save(os.Getenv("NOGUI") != "")
				panic(msg + " — intentional crash, restart manually")
			} else {
				println(msg)
			}
		} else if strings.Contains(pngFilePath, "/bwsmall.") && strings.HasSuffix(pngFilePath, "."+itoa(int(App.Proj.Sheets.Bw.SmallWidth))+".png") {
			if hashid := filepath.Base(filepath.Dir(pngFilePath)); App.Proj.data.Sv.ById != nil {
				if svdata := App.Proj.data.Sv.ById[hashid]; svdata != nil && svdata.parentSheetVer != nil {
					_ = os.Remove(filepath.Join(svdata.DirPath, strings.TrimSuffix(
						filepath.Base(svdata.parentSheetVer.FileName), ".png")+".svg"))
				}
			}
		}
		return true
	}
	return false
}
