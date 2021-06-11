package main

import (
	"os"
	"path/filepath"
)

var App struct {
	StaticFilesDirPath string
	Proj               Project
	Gui                struct {
		BrowserClosed bool
		State         struct {
			SelectedSeries  *Series
			SelectedChapter *Chapter
			SelectedSheet   *Sheet
			SelectedVersion *SheetVersion
		}
	}
}

func appInit() {
	App.StaticFilesDirPath = filepath.Join(os.Getenv("HOME"), "c/go/src/github.com/metaleap/cositegen/_static")
	if err := os.Mkdir(".build", os.ModePerm); err != nil && !os.IsExist(err) {
		panic(err)
	}
	App.Gui.State.SelectedChapter = nil
	App.Gui.State.SelectedSheet = nil
	App.Gui.State.SelectedSeries = nil
	App.Proj.Load("cosite.json")
}

func appMainAction(name string) string {
	switch name {
	case "regen_site":
	default:
		return "Unknown action: " + name
	}

	return ""
}
