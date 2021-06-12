package main

import (
	"fmt"
	"html"
	"net/http"
)

var hEsc = html.EscapeString

func guiMain(r *http.Request, notice string) []byte {
	rVal := r.FormValue
	s := "<html><head><link rel='stylesheet' href='/styles.css'/>"
	s += "</head><body><form id='main_form'><input type='hidden' name='main_focus_id' id='main_focus_id' value='" + hEsc(rVal("main_focus_id")) + "'/>"

	if notice != "" {
		s += "<div class='notice'>" + hEsc(notice) + "</div>"
	}

	App.Gui.State.SelectedSeries, _ = guiGetFormSel(rVal("series"), &App.Proj).(*Series)
	s += guiHtmlList("series", "(Series)", len(App.Proj.Series), func(i int) (string, string, bool) {
		return App.Proj.Series[i].Name, App.Proj.Series[i].Title, App.Gui.State.SelectedSeries != nil && App.Proj.Series[i].Name == App.Gui.State.SelectedSeries.Name
	})

	if series := App.Gui.State.SelectedSeries; series != nil {
		App.Gui.State.SelectedChapter, _ = guiGetFormSel(rVal("chapter"), series).(*Chapter)
		s += guiHtmlList("chapter", "(Chapters)", len(series.Chapters), func(i int) (string, string, bool) {
			chapter := series.Chapters[i]
			return chapter.Name, chapter.Title, App.Gui.State.SelectedChapter != nil && App.Gui.State.SelectedChapter.Name == chapter.Name
		})
		if chapter := App.Gui.State.SelectedChapter; chapter != nil {
			App.Gui.State.SelectedSheet, _ = guiGetFormSel(rVal("sheet"), chapter).(*Sheet)
			s += guiHtmlList("sheet", "(Sheets)", len(chapter.sheets), func(i int) (string, string, bool) {
				sheet := chapter.sheets[i]
				return sheet.name, sheet.name, App.Gui.State.SelectedSheet != nil && App.Gui.State.SelectedSheet.name == sheet.name
			})
			if sheet := App.Gui.State.SelectedSheet; sheet != nil {
				App.Gui.State.SelectedVersion, _ = guiGetFormSel(rVal("sheetver"), sheet).(*SheetVersion)
				s += guiHtmlList("sheetver", "(Versions)", len(sheet.versions), func(i int) (string, string, bool) {
					sheetver := sheet.versions[i]
					return sheetver.fileName, sheetver.name, App.Gui.State.SelectedVersion != nil && App.Gui.State.SelectedVersion.fileName == sheetver.fileName
				})
				if sheetver := App.Gui.State.SelectedVersion; sheetver != nil {
					s += guiSheet(sheetver)
				}
			}
		}
	}

	s += "<hr/>" + guiHtmlListFrom("main_action", "(Actions)", map[string]string{
		"regen_site": "ReGen Site",
	})

	s += "</form></body>"
	if rVal("main_focus_id") != "main_action" {
		s += "<script type='text/javascript'>try { document.getElementById(\"" + rVal("main_focus_id") + "\").focus() } catch (ignore) {}</script></html>"
	}
	return []byte(s)
}

func guiSheet(sv *SheetVersion) string {
	sv.ensureFullMeta(true)
	s := "<hr/>" + guiHtmlImg("/"+sv.fileName)
	return s
}

func guiHtmlImg(uri string) string {
	s := "<img src='" + hEsc(uri) + "'/>"
	return s
}

func guiHtmlList(name string, noneItemFirst string, numItems int, getItem func(int) (string, string, bool)) string {
	s := "<select onchange='document.getElementById(\"main_focus_id\").value = \"" + hEsc(name) + "\"; document.getElementById(\"main_form\").submit()' name='" + hEsc(name) + "' id='" + hEsc(name) + "'>"
	if noneItemFirst != "" {
		s += "<option value=''>" + noneItemFirst + "</option>"
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

func guiHtmlListFrom(name string, noneItemFirst string, from map[string]string) string {
	var keys, vals []string
	for k, v := range from {
		keys, vals = append(keys, k), append(vals, v)
	}
	return guiHtmlList(name, noneItemFirst, len(from), func(i int) (string, string, bool) { return keys[i], vals[i], false })
}

func guiGetFormSel(formVal string, slice Indexed) fmt.Stringer {
	for i, l := 0, slice.Len(); i < l; i++ {
		if item := slice.At(i); item != nil && item.String() == formVal {
			return item
		}
	}
	return nil
}
