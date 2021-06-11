package main

import (
	"html"
	"net/http"
)

var hEsc = html.EscapeString

func guiMain(r *http.Request) []byte {
	s := "<html><head><link rel='stylesheet' href='/styles.css'/>"
	s += "</head><body><form id='main_form'>"

	App.Gui.State.SelectedSeries = nil
	for _, series := range App.Proj.Series {
		if series.Name == r.FormValue("series") {
			App.Gui.State.SelectedSeries = series
			break
		}
	}
	s += guiList("series", true, len(App.Proj.Series), func(i int) (string, string, bool) {
		return App.Proj.Series[i].Name, App.Proj.Series[i].Title, App.Gui.State.SelectedSeries != nil && App.Proj.Series[i].Name == App.Gui.State.SelectedSeries.Name
	})

	if App.Gui.State.SelectedSeries != nil {
		App.Gui.State.SelectedChapter = nil
		for _, Chapter := range App.Gui.State.SelectedSeries.Chapters {
			if Chapter.Name == r.FormValue("chapter") {
				App.Gui.State.SelectedChapter = Chapter
				break
			}
		}
		s += guiList("chapter", true, len(App.Gui.State.SelectedSeries.Chapters), func(i int) (string, string, bool) {
			chapter := App.Gui.State.SelectedSeries.Chapters[i]
			return chapter.Name, chapter.Title, App.Gui.State.SelectedChapter != nil && App.Gui.State.SelectedChapter.Name == chapter.Name
		})
	}

	s += "</form></body></html>"
	return []byte(s)
}

func guiList(name string, noneItemFirst bool, numItems int, getItem func(int) (string, string, bool)) string {
	s := "<select onchange='document.getElementById(\"main_form\").submit()' name='" + hEsc(name) + "' id='" + hEsc(name) + "'>"
	if noneItemFirst {
		s += "<option>(None)</option>"
	}
	for i := 0; i < numItems; i++ {
		value, caption, sel := getItem(i)
		if s += "<option value='" + value + "'"; sel {
			s += " selected"
		}
		s += ">" + hEsc(caption) + "</option>"
	}
	s += "</select>"
	return s
}
