package main

import (
	"errors"
	"image/color"
	"io/fs"
	"os"
	"os/exec"
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
	App.Proj.save()
	rmDir(".csg/tmp")
}

func appMainAction(fromGui bool, name string, args map[string]bool) string {
	if appMainActions[name] {
		return "Action '" + name + "' already in progress and not yet done."
	}
	appMainActions[name] = true

	var action func(map[string]bool)
	switch name {
	case "sitegen", "now":
		action = func(flags map[string]bool) { siteGen{}.genSite(fromGui, flags) }
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
	} else {
		action(args)
		return ""
	}
}

func appPrepWork(fromGui bool) {
	App.Proj.allPrepsDone = false
	timedLogged("Preprocessing...", func() string {
		var numjobs, numwork int
		for _, series := range App.Proj.Series {
			var thumbsrcfilenames []string
			var didanywork bool
			for _, chapter := range series.Chapters {
				for _, sheet := range chapter.sheets {
					for i, sv := range sheet.versions {
						if !sv.prep.done {
							sv.prep.Lock()
							if !sv.prep.done {
								didwork := sv.ensurePrep(true, false)
								if sv.prep.done, numjobs = true, numjobs+1; didwork {
									numwork, didanywork = numwork+1, true
								}
							}
							sv.prep.Unlock()
						}
						if i == 0 {
							thumbsrcfilenames = append(thumbsrcfilenames, sv.data.bwSmallFilePath)
						}
					}
				}
			}
			thumbfilepath := ".csg/sv/" + siteGen{}.nameThumb(series) + ".png"
			if didanywork || len(thumbsrcfilenames) == 0 || App.Proj.NumSheetsInHomeBgs == 0 {
				_ = os.Remove(thumbfilepath)
			}
			if len(thumbsrcfilenames) > 0 && App.Proj.NumSheetsInHomeBgs > 0 &&
				(didanywork || nil == statFileOnly(thumbfilepath)) {
				if len(thumbsrcfilenames) > App.Proj.NumSheetsInHomeBgs {
					thumbsrcfilenames = thumbsrcfilenames[len(thumbsrcfilenames)-App.Proj.NumSheetsInHomeBgs:]
				}
				writeFile(thumbfilepath, imgStitchHorizontally(thumbsrcfilenames, 320, 44, color.NRGBA{0, 0, 0, 0}))
			}
		}
		App.Proj.allPrepsDone = true
		return "for " + itoa(numwork) + "/" + itoa(numjobs) + " preprocessing jobs"
	})
	if fromGui {
		pngOptsLoop()
	}
}

func pngOptsLoop() {
	App.pngOptBusy = true
	defer func() { App.pngOptBusy = false }()

	for dirfs := os.DirFS("."); !App.Gui.Exiting; time.Sleep(time.Minute) {
		dels := false
		for k := range App.Proj.data.PngOpt {
			if statFileOnly(k) == nil {
				delete(App.Proj.data.PngOpt, k)
				dels = true
			}
		}
		if dels {
			App.Proj.save()
		}
		if App.Gui.Exiting {
			return
		}

		numdone, matches, totalsize, errexiting := 0, FilePathsSortingByFileSize{}, uint64(0), errors.New("exiting")
		if err := fs.WalkDir(dirfs, ".", func(fspath string, dir fs.DirEntry, err error) error {
			if App.Gui.Exiting {
				return errexiting
			}
			if fileinfo, err := os.Lstat(fspath); err == nil &&
				(!fileinfo.IsDir()) && (fileinfo.Mode()&os.ModeSymlink) == 0 &&
				strings.HasSuffix(fspath, ".png") && !strings.Contains(fspath, ".build/") {
				matches, totalsize = append(matches, fspath), totalsize+uint64(fileinfo.Size())
			}
			return nil
		}); err == errexiting {
			return
		} else if err != nil {
			printLn("PNGOPT Walk: " + err.Error())
		}
		sort.Sort(matches)

		printLn("PNGOPT: found", len(matches), "files (~"+itoa(int(totalsize/(1024*1024)))+"MB) to scrutinize...")
		for _, pngfilename := range matches {
			if App.Gui.Exiting {
				return
			}
			curfiledata := readFile(pngfilename)
			lastopt, skip := App.Proj.data.PngOpt[pngfilename]
			if skip = skip && (lastopt[1] == itoa(len(curfiledata))) &&
				(lastopt[2] == string(contentHashStr(curfiledata))); skip {
				continue
			} else if App.Gui.Exiting {
				return
			}

			cmd := exec.Command("pngbattle", pngfilename)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Start(); err != nil {
				printLn(err)
				continue
			}
			go cmd.Wait()
			for ; cmd.ProcessState == nil; time.Sleep(time.Second) {
				if App.Gui.Exiting {
					_ = cmd.Process.Kill()
				}
			}
			if !cmd.ProcessState.Success() {
				printLn(cmd.ProcessState.String())
				continue
			}
			if filedata, err := os.ReadFile(pngfilename); err == nil {
				numdone, App.Proj.data.PngOpt[pngfilename] = numdone+1, []string{
					itoa(len(curfiledata)),
					itoa(len(filedata)),
					string(contentHashStr(filedata)),
				}
				App.Proj.save()
			}
		}
		printLn("PNGOPT:", len(matches), "scrutinized &", numdone, "processed, sleeping a minute...")
	}
}

type FilePathsSortingByFileSize []string

func (me FilePathsSortingByFileSize) Len() int          { return len(me) }
func (me FilePathsSortingByFileSize) Swap(i int, j int) { me[i], me[j] = me[j], me[i] }
func (me FilePathsSortingByFileSize) Less(i int, j int) bool {
	fi1, fi2 := statFileOnly(me[i]), statFileOnly(me[j])
	return fi1 == nil || fi2 == nil || fi1.Size() < fi2.Size()
}
