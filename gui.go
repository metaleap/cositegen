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

	App.Gui.State.Sel.Series, _ = guiGetFormSel(rVal("series"), &App.Proj).(*Series)
	s += guiHtmlList("series", "(Series)", len(App.Proj.Series), func(i int) (string, string, bool) {
		return App.Proj.Series[i].Name, App.Proj.Series[i].Title, App.Gui.State.Sel.Series != nil && App.Proj.Series[i].Name == App.Gui.State.Sel.Series.Name
	})

	if series := App.Gui.State.Sel.Series; series != nil {
		App.Gui.State.Sel.Chapter, _ = guiGetFormSel(rVal("chapter"), series).(*Chapter)
		s += guiHtmlList("chapter", "(Chapters)", len(series.Chapters), func(i int) (string, string, bool) {
			chapter := series.Chapters[i]
			return chapter.Name, chapter.Title, App.Gui.State.Sel.Chapter != nil && App.Gui.State.Sel.Chapter.Name == chapter.Name
		})
		if chapter := App.Gui.State.Sel.Chapter; chapter != nil {
			App.Gui.State.Sel.Sheet, _ = guiGetFormSel(rVal("sheet"), chapter).(*Sheet)
			s += guiHtmlList("sheet", "(Sheets)", len(chapter.sheets), func(i int) (string, string, bool) {
				sheet := chapter.sheets[i]
				return sheet.name, sheet.name, App.Gui.State.Sel.Sheet != nil && App.Gui.State.Sel.Sheet.name == sheet.name
			})
			if sheet := App.Gui.State.Sel.Sheet; sheet != nil {
				App.Gui.State.Sel.Ver, _ = guiGetFormSel(rVal("sheetver"), sheet).(*SheetVer)
				s += guiHtmlList("sheetver", "(Versions)", len(sheet.versions), func(i int) (string, string, bool) {
					sheetver := sheet.versions[i]
					return sheetver.fileName, sheetver.name, App.Gui.State.Sel.Ver != nil && App.Gui.State.Sel.Ver.fileName == sheetver.fileName
				})
				if sheetver := App.Gui.State.Sel.Ver; sheetver != nil {
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

func guiSheet(sv *SheetVer) string {
	sv.ensure(true)
	s := "<hr/><h3>Full Sheet:</h3><div>" + guiHtmlImg("/"+sv.meta.bwFilePath) + "</div>"
	var panelstree func(*ImgPanel) string
	panelstree = func(panel *ImgPanel) (s string) {
		assert(len(panel.SubCols) == 0 || len(panel.SubRows) == 0)
		if len(panel.SubRows) > 0 {
			s += "<ul>"
			for _, row := range panel.SubRows {
				s += "<li><div>Row: " + row.Rect.String() + "</div>" + panelstree(&row) + "</li>"
			}
			s += "</ul>"
		} else if len(panel.SubCols) > 0 {
			s += "<ul>"
			for _, col := range panel.SubCols {
				s += "<li><div>Col: " + col.Rect.String() + "</div>" + panelstree(&col) + "</li>"
			}
			s += "</ul>"
		} else {
			s += "<ul><li><div>Panel: " + panel.Rect.String() + "</div></li></ul>"
		}
		return
	}
	s += "<h3>Sheet Panels Structure:</h3><div>" + panelstree(sv.meta.PanelsTree) + "</div>"
	pidx := 0
	sv.meta.PanelsTree.iter(func(panel *ImgPanel) {
		rect := panel.Rect
		style := `background-image: url("` + sv.meta.bwFilePath + `");`
		style += `width: ` + itoa(rect.Max.X-rect.Min.X) + `px; height: ` + itoa(rect.Max.Y-rect.Min.Y) + `px;`
		s += "<h3>Panel #" + itoa(pidx+1) + "</h3>"
		s += "<div class='.panel' style='" + style + "'></div>"
		s += rect.String()
		pidx++
	})
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
