package main

import (
	"fmt"
	"html"
	"net/http"
)

var hEsc = html.EscapeString

func guiMain(r *http.Request) []byte {
	s := "<html><head><link rel='stylesheet' href='/styles.css'/>"
	s += "</head><body><form id='main_form'>"

	App.Gui.State.SelectedSeries, _ = guiGetFormSel(r.FormValue("series"), &App.Proj).(*Series)
	s += guiList("series", "(Series)", len(App.Proj.Series), func(i int) (string, string, bool) {
		return App.Proj.Series[i].Name, App.Proj.Series[i].Title, App.Gui.State.SelectedSeries != nil && App.Proj.Series[i].Name == App.Gui.State.SelectedSeries.Name
	})

	if series := App.Gui.State.SelectedSeries; series != nil {
		App.Gui.State.SelectedChapter, _ = guiGetFormSel(r.FormValue("chapter"), series).(*Chapter)
		s += guiList("chapter", "(Chapters)", len(series.Chapters), func(i int) (string, string, bool) {
			chapter := series.Chapters[i]
			return chapter.Name, chapter.Title, App.Gui.State.SelectedChapter != nil && App.Gui.State.SelectedChapter.Name == chapter.Name
		})
		if chapter := App.Gui.State.SelectedChapter; chapter != nil {
			App.Gui.State.SelectedScan, _ = guiGetFormSel(r.FormValue("scan"), chapter).(*Scan)
			s += guiList("scan", "(Scans)", len(chapter.scans), func(i int) (string, string, bool) {
				scan := chapter.scans[i]
				return scan.name, scan.name, false
			})
		}
	}

	s += "<hr/>" + guiListFrom("main_action", "(Actions)", map[string]string{
		"regen_site":  "ReGen Site",
		"reload_proj": "Reload Proj",
	})

	s += "</form></body></html>"
	return []byte(s)
}

func guiList(name string, noneItemFirst string, numItems int, getItem func(int) (string, string, bool)) string {
	s := "<select onchange='document.getElementById(\"main_form\").submit()' name='" + hEsc(name) + "' id='" + hEsc(name) + "'>"
	if noneItemFirst != "" {
		s += "<option>" + noneItemFirst + "</option>"
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

func guiListFrom(name string, noneItemFirst string, from map[string]string) string {
	var keys, vals []string
	for k, v := range from {
		keys, vals = append(keys, k), append(vals, v)
	}
	return guiList(name, noneItemFirst, len(from), func(i int) (string, string, bool) { return keys[i], vals[i], false })
}

func guiGetFormSel(formVal string, slice Indexed) fmt.Stringer {
	for i, l := 0, slice.Len(); i < l; i++ {
		if item := slice.At(i); item != nil && item.String() == formVal {
			return item
		}
	}
	return nil
}
