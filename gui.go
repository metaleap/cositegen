package main

import (
	"fmt"
	"html"
	"image"
	"net/http"
)

var hEsc = html.EscapeString

func guiMain(r *http.Request, notice string) []byte {
	rVal := r.FormValue
	s := "<html><head><link rel='stylesheet' href='/main.css'/><script src='/main.js' type='text/javascript' language='javascript'></script>"
	s += "</head><body><form id='main_form'>" + guiHtmlInput("hidden", "main_focus_id", rVal("main_focus_id"), nil)
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

	s += "<hr/>" + guiHtmlListFrom("main_action", "(Actions)", A{"regen_site": "ReGen Site"})

	s += "</form></body>"
	if rVal("main_focus_id") != "main_action" {
		s += "<script type='text/javascript'>try { document.getElementById(\"" + rVal("main_focus_id") + "\").focus(); } catch (ignore) {}</script></html>"
	}
	return []byte(s)
}

func guiSheet(sv *SheetVer) string {
	sv.ensure(true)
	s := "<hr/><h3>Full Sheet:</h3><div class='fullsheet'>" + guiHtmlImg("/"+sv.meta.bwSmallFilePath) + "</div>"
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
	s += "<h3>Sheet Panels Structure:</h3><ul><li>Sheet:" + sv.meta.PanelsTree.Rect.String() + panelstree(sv.meta.PanelsTree) + "</li></ul>"
	pidx, maxwidth, zoom := 0, 0, 100
	sv.meta.PanelsTree.iter(func(panel *ImgPanel) {
		if w := panel.Rect.Max.X - panel.Rect.Min.X; w > maxwidth {
			maxwidth = w
		}
	})
	if wmax := 700; maxwidth > wmax {
		zoomdiv := float64(maxwidth) / float64(wmax)
		zoom = int(100.0 / zoomdiv)
	}
	sv.meta.PanelsTree.iter(func(panel *ImgPanel) {
		rect, pid := panel.Rect, "p"+itoa(pidx)
		w, h := rect.Max.X-rect.Min.X, rect.Max.Y-rect.Min.Y
		s += "<h3>Panel #" + itoa(pidx+1) + "</h3><div>" + rect.String() + "</div>"

		style := `background-image: url("/` + sv.meta.bwSmallFilePath + `");`
		style += `background-size: ` + itoa(sv.meta.PanelsTree.Rect.Max.X-sv.meta.PanelsTree.Rect.Min.X) + `px ` + itoa(sv.meta.PanelsTree.Rect.Max.Y-sv.meta.PanelsTree.Rect.Min.Y) + `px;`
		style += `width: ` + itoa(w) + `px; height: ` + itoa(h) + `px;`
		style += `background-position: -` + itoa(rect.Min.X) + `px -` + itoa(rect.Min.Y) + `px;`
		s += "<table><tr><td>"
		s += "<div class='panel' style='zoom: " + itoa(zoom) + "%;' onclick='toggle(\"" + pid + "cfg\")'><div style='" + style + "'></div></div>"
		s += "</td><td>"
		s += "<div class='panelcfg' id='" + pid + "cfg' style='display:none;'>"
		for i := 0; i < 8; i++ {
			texts, rect := A{}, image.ZR
			if len(panel.Areas) > i {
				texts, rect = panel.Areas[i].Data, panel.Areas[i].Rect
			}
			for _, ptk := range App.Proj.PanelTextKinds {
				s += "<div>" + guiHtmlInput("textarea", pid+"t"+itoa(i), texts[ptk], A{"placeholder": ptk, "class": "panelcfgtext col" + itoa(i)}) + "</div><div>"
			}
			s += "Min X,Y:"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rx0", itoa(rect.Min.X), A{"class": "panelcfgrect"})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"ry0", itoa(rect.Min.Y), A{"class": "panelcfgrect"})
			s += "Max X,Y:"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rx1", itoa(rect.Max.X), A{"class": "panelcfgrect"})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"ry1", itoa(rect.Max.Y), A{"class": "panelcfgrect"})
			s += "</div>"
		}
		s += guiHtmlButton(pid+"save", "Save")
		s += guiHtmlButton(pid+"reset", "Reset")
		s += "</div>"
		s += "</td></tr></table>"
		pidx++
	})
	return s
}

func guiHtmlImg(uri string) string {
	s := "<img src='" + hEsc(uri) + "'/>"
	return s
}

func guiHtmlList(name string, noneItemFirst string, numItems int, getItem func(int) (string, string, bool)) string {
	s := "<select onchange='doPostBack(\"" + hEsc(name) + "\");' name='" + hEsc(name) + "' id='" + hEsc(name) + "'>"
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

func guiHtmlButton(id string, text string) string {
	s := "<button id='" + hEsc(id) + "'>" + hEsc(text) + "</button>"
	return s
}

func guiHtmlInput(inputType string, id string, value string, attrs map[string]string) string {
	s := "<input name='" + hEsc(id) + "' id='" + hEsc(id) + "' type='" + hEsc(inputType) + "' value='" + hEsc(value) + "'"
	if inputType == "textarea" {
		s = "<textarea name='" + hEsc(id) + "' id='" + hEsc(id) + "'"
	}
	if attrs != nil {
		for k, v := range attrs {
			s += " " + hEsc(k) + "='" + hEsc(v) + "'"
		}
	}
	if inputType == "textarea" {
		s += ">" + hEsc(value) + "</textarea>"
	} else {
		s += "/>"
	}
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
