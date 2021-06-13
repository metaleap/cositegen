package main

import (
	"image"
	"net/http"
	"strconv"
	"strings"
)

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
					html, shouldsavemeta := guiSheet(sheetver, r)
					s += html
					if shouldsavemeta {
						App.Proj.save()
					}
				}
			}
		}
	}

	s += "<hr/>" + guiHtmlListFrom("main_action", "(Actions)", A{"regen_site": "ReGen Site"})

	s += "</form></body>"
	if rfv := rVal("main_focus_id"); rfv != "" && rfv != "main_action" && notice == "" {
		s += "<script language='javascript' type='text/javascript'>try { document.getElementById(\"" + rfv + "\").focus(); } catch (ignore) {}</script></html>"
	}
	return []byte(s)
}

func guiSheet(sv *SheetVer, r *http.Request) (s string, shouldSaveMeta bool) {
	sv.ensure(true)
	s = "<hr/><h3>Full Sheet:</h3><div class='fullsheet'>" + guiHtmlImg("/"+sv.meta.bwSmallFilePath, nil) + "</div>"
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
	pidx, numpanels, maxwidth, zoom := 0, 0, 0, 100
	sv.meta.PanelsTree.iter(func(panel *ImgPanel) {
		numpanels++
		if w := panel.Rect.Max.X - panel.Rect.Min.X; w > maxwidth {
			maxwidth = w
		}
	})
	if wmax := 700; maxwidth > wmax {
		zoomdiv := float64(maxwidth) / float64(wmax)
		zoom = int(100.0 / zoomdiv)
	}
	if rfv := r.FormValue("main_focus_id"); rfv != "" && rfv[0] == 'p' && strings.HasSuffix(rfv, "save") {
		shouldSaveMeta = true
	}
	sv.meta.PanelsTree.iter(func(panel *ImgPanel) {
		rect, pid := panel.Rect, "p"+itoa(pidx)
		w, h := rect.Max.X-rect.Min.X, rect.Max.Y-rect.Min.Y
		cfgdisplay := "none"
		if shouldSaveMeta {
			panel.Areas = nil
			if r.FormValue("main_focus_id") == pid+"save" {
				cfgdisplay = "block"
			}
			for i := 0; i < MaxImagePanelAreas; i++ {
				hastexts, area := false, ImgPanelArea{Data: A{}}
				for _, ptk := range App.Proj.PanelTextKinds {
					tid := pid + "t" + itoa(i) + ptk
					if tval := r.FormValue(tid); tval != "" {
						hastexts, area.Data[ptk] = true, tval
					}
				}
				if hastexts {
					trx0, trx1 := r.FormValue(pid+"t"+itoa(i)+"rx0"), r.FormValue(pid+"t"+itoa(i)+"rx1")
					try0, try1 := r.FormValue(pid+"t"+itoa(i)+"ry0"), r.FormValue(pid+"t"+itoa(i)+"ry1")
					rx0, _ := strconv.ParseUint(trx0, 0, 64)
					rx1, _ := strconv.ParseUint(trx1, 0, 64)
					ry0, _ := strconv.ParseUint(try0, 0, 64)
					ry1, _ := strconv.ParseUint(try1, 0, 64)
					area.Rect = image.Rect(int(rx0), int(ry0), int(rx1), int(ry1))
					panel.Areas = append(panel.Areas, area)
				}
			}
		}
		s += "<h3>Panel #" + itoa(pidx+1) + " (" + itoa(len(panel.Areas)) + ")" + "</h3><div>" + rect.String() + "</div>"

		s += "<table><tr><td>"
		s += "<div class='panel' style='zoom: " + itoa(zoom) + "%;' onclick='toggle(\"" + pid + "cfg\")'>"
		style := `width: ` + itoa(w) + `px; height: ` + itoa(h) + `px;`
		s += "<div style='position:relative; " + style + "'>"
		style += `background-image: url("/` + sv.meta.bwSmallFilePath + `");`
		style += `background-size: ` + itoa(sv.meta.PanelsTree.Rect.Max.X-sv.meta.PanelsTree.Rect.Min.X) + `px ` + itoa(sv.meta.PanelsTree.Rect.Max.Y-sv.meta.PanelsTree.Rect.Min.Y) + `px;`
		style += `background-position: -` + itoa(rect.Min.X) + `px -` + itoa(rect.Min.Y) + `px;`
		s += "<div class='panelpic' style='" + style + "'></div><span id='" + pid + "rects'></span>"
		s += "</div></div></td><td>"

		s += "<div class='panelcfg' id='" + pid + "cfg' style='display:" + cfgdisplay + ";'>"
		jsrefr, savebtnhtml := "refreshPanelRects("+itoa(pidx)+", "+itoa(panel.Rect.Min.X)+", "+itoa(panel.Rect.Min.Y)+", "+itoa(MaxImagePanelAreas)+", [\""+strings.Join(App.Proj.PanelTextKinds, "\", \"")+"\"]);", guiHtmlButton(pid+"save", "Save changes (to all texts in all panels)", A{"onclick": "doPostBack(\"" + pid + "save\")"})
		s += savebtnhtml + "<hr/>"
		for i := 0; i < MaxImagePanelAreas; i++ {
			area := ImgPanelArea{Data: A{}}
			if len(panel.Areas) > i {
				area = panel.Areas[i]
			}
			for _, ptk := range App.Proj.PanelTextKinds {
				s += "<div>" + guiHtmlInput("textarea", pid+"t"+itoa(i)+ptk, area.Data[ptk], A{"placeholder": ptk, "onchange": jsrefr, "onfocus": jsrefr, "class": "panelcfgtext col" + itoa(i)}) + "</div><div>"
			}
			s += "X,Y:"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rx0", itoa(area.Rect.Min.X), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.X), "max": itoa(panel.Rect.Max.X)})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"ry0", itoa(area.Rect.Min.Y), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.Y), "max": itoa(panel.Rect.Max.Y)})
			s += "X,Y:"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rx1", itoa(area.Rect.Max.X), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.X), "max": itoa(panel.Rect.Max.X)})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"ry1", itoa(area.Rect.Max.Y), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.Y), "max": itoa(panel.Rect.Max.Y)})
			s += "</div>"
		}
		s += "<hr/>" + savebtnhtml
		s += "</div>"
		s += "</td></tr></table>"
		s += "<script language='javascript' type='text/javascript'>" + jsrefr + "</script>"
		pidx++
	})
	return
}
