package main

import (
	"image"
	"net/http"
	"strconv"
	"strings"
)

var wthDisAintWindoze = strings.NewReplacer("\r\n", "\n")

func guiMain(r *http.Request, notice string) []byte {
	rVal := func(s string) string { return wthDisAintWindoze.Replace(r.FormValue(s)) }

	s := "<!DOCTYPE html><html><head><link rel='stylesheet' type='text/css' href='/main.css'/><style type='text/css'>"
	for csssel, csslines := range App.Proj.Gen.PanelSvgText.Css {
		if csssel == "" {
			csssel = "div.panel .panelrect svg text"
		}
		if csslines != nil {
			s += csssel + "{" + strings.Join(csslines, ";") + "}"
		}
	}
	s += "</style><script type='text/javascript' language='javascript'>const svgTxtPerLineDyCmA4 = " + strconv.FormatFloat(App.Proj.Gen.PanelSvgText.PerLineDyCmA4, 'f', 8, 64) + ", svgTxtFontSizeCmA4 = " + strconv.FormatFloat(App.Proj.Gen.PanelSvgText.FontSizeCmA4, 'f', 8, 64) + ";</script><script src='/main.js' type='text/javascript' language='javascript'></script>"
	s += "</head><body><form method='POST' action='/' id='main_form'>" + guiHtmlInput("hidden", "main_focus_id", rVal("main_focus_id"), nil)
	if notice != "" {
		s += "<div class='notice'>" + hEsc(notice) + "</div>"
	}

	App.Gui.State.Sel.Series, _ = guiGetFormSel(rVal("series"), &App.Proj).(*Series)
	s += guiHtmlList("series", "(Series)", len(App.Proj.Series), func(i int) (string, string, bool) {
		return App.Proj.Series[i].Name, App.Proj.Series[i].Title["en"], App.Gui.State.Sel.Series != nil && App.Proj.Series[i].Name == App.Gui.State.Sel.Series.Name
	})

	if series := App.Gui.State.Sel.Series; series != nil {
		App.Gui.State.Sel.Chapter, _ = guiGetFormSel(rVal("chapter"), series).(*Chapter)
		s += guiHtmlList("chapter", "(Chapters)", len(series.Chapters), func(i int) (string, string, bool) {
			chapter := series.Chapters[i]
			return chapter.Name, chapter.Title["en"], App.Gui.State.Sel.Chapter != nil && App.Gui.State.Sel.Chapter.Name == chapter.Name
		})
		if chapter := App.Gui.State.Sel.Chapter; chapter != nil {
			App.Gui.State.Sel.Sheet, _ = guiGetFormSel(rVal("sheet"), chapter).(*Sheet)
			s += guiHtmlList("sheet", "(Sheets)", len(chapter.sheets), func(i int) (string, string, bool) {
				sheet := chapter.sheets[i]
				return sheet.name, sheet.name, App.Gui.State.Sel.Sheet != nil && App.Gui.State.Sel.Sheet.name == sheet.name
			})
			if sheet := App.Gui.State.Sel.Sheet; sheet == nil {
				s += guiSheetScan(r)
			} else if len(sheet.versions) > 0 {
				App.Gui.State.Sel.Ver, _ = guiGetFormSel(rVal("sheetver"), sheet).(*SheetVer)
				s += guiHtmlList("sheetver", "", len(sheet.versions), func(i int) (string, string, bool) {
					sheetver := sheet.versions[i]
					return sheetver.fileName, sheetver.name, App.Gui.State.Sel.Ver != nil && App.Gui.State.Sel.Ver.fileName == sheetver.fileName
				})
				if App.Gui.State.Sel.Ver == nil {
					App.Gui.State.Sel.Ver = sheet.versions[0]
				}
				if sheetver := App.Gui.State.Sel.Ver; sheetver != nil {
					html, shouldsavemeta := guiSheetEdit(sheetver, r)
					s += html
					if shouldsavemeta {
						App.Proj.save()
					}
				}
			}
		}
	}

	s += "<hr/>" + guiHtmlListFrom("main_action", "(Project Actions)", AppMainActions)

	s += "</form></body>"
	if rfv := rVal("main_focus_id"); rfv != "" && rfv != "main_action" && notice == "" {
		s += "<script language='javascript' type='text/javascript'>try { document.getElementById(\"" + rfv + "\").focus(); } catch (ignore) {}</script></html>"
	}
	return []byte(s)
}

func guiSheetScan(r *http.Request) (s string) {
	s = "<hr><h3>New Sheet Version Scan</h3>"
	s += guiHtmlInput("text", "sheetname", "", A{"placeholder": "Sheet Name"})
	s += guiHtmlInput("text", "sheetvername", "", A{"placeholder": "Sheet Version Name"})
	s += "<h4>Scanner To Use:</h4>"

	if len(scannerDevices) == 1 {
		tmpcopy := *scannerDevices[0]
		tmpcopy.Dev += "copy"
		tmpcopy.Model += "Copy"
		tmpcopy.Nr--
		scannerDevices = append(scannerDevices, &tmpcopy)
	}

	s += "<div><select onchange='toggleScanOpt(this.options[this.selectedIndex].value)'>"
	for i, sd := range scannerDevices {
		htmlsel := " selected='selected'"
		if i != len(scannerDevices)-1 {
			htmlsel = ""
		}
		s += "<option value='" + sd.Dev + "'" + htmlsel + ">" + hEsc(sd.String()) + "</option>"
	}
	s += "</select></div><div class='scandevoptsbox'>"
	for i, sd := range scannerDevices {
		cssdisplay := "block"
		if i != len(scannerDevices)-1 {
			cssdisplay = "none"
		}
		s += "<div class='scandevopts' id='scandevopts_" + sd.Dev + "' style='display: " + cssdisplay + "'>"
		s += "(options for " + sd.Dev + ")"
		s += "</div>"
	}
	s += "</div>"
	return
}

func guiSheetEdit(sv *SheetVer, r *http.Request) (s string, shouldSaveMeta bool) {
	rVal := func(s string) string { return wthDisAintWindoze.Replace(r.FormValue(s)) }

	sv.ensurePrep(false, false)
	px1cm := float64(sv.meta.PanelsTree.Rect.Max.Y-sv.meta.PanelsTree.Rect.Min.Y) / 21.0
	s = "<hr/><h3>Full Sheet:</h3><div class='fullsheet'>" + guiHtmlImg("/"+sv.meta.bwSmallFilePath, nil) + "</div>"
	var panelstree func(*ImgPanel) string
	panelstree = func(panel *ImgPanel) (s string) {
		assert(len(panel.SubCols) == 0 || len(panel.SubRows) == 0)
		if len(panel.SubRows) > 0 {
			s += "<ul>"
			for i := range panel.SubRows {
				s += "<li><div>Row: " + panel.SubRows[i].Rect.String() + "</div>" + panelstree(&panel.SubRows[i]) + "</li>"
			}
			s += "</ul>"
		} else if len(panel.SubCols) > 0 {
			s += "<ul>"
			for i := range panel.SubCols {
				s += "<li><div>Col: " + panel.SubCols[i].Rect.String() + "</div>" + panelstree(&panel.SubCols[i]) + "</li>"
			}
			s += "</ul>"
		} else {
			s += "<ul><li><div>Panel: " + panel.Rect.String() + "</div></li></ul>"
		}
		return
	}
	s += "<h3>Sheet Panels Structure:</h3><ul><li>Sheet coords:" + sv.meta.PanelsTree.Rect.String() + panelstree(sv.meta.PanelsTree) + "</li></ul><hr/><h3>All panels:</h3>"
	pidx, numpanels, maxwidth, zoom, zoomdiv := 0, 0, 0, 100, 1.0
	sv.meta.PanelsTree.iter(func(panel *ImgPanel) {
		numpanels++
		if w := panel.Rect.Max.X - panel.Rect.Min.X; w > maxwidth {
			maxwidth = w
		}
	})
	for i, lang := range App.Proj.Langs {
		attrs := A{"name": "plang", "onclick": "refreshAllPanelRects(" + itoa(numpanels) + "," + itoa(i) + ",\"" + lang + "\");"}
		if i == 0 {
			attrs["checked"] = "checked"
		}
		s += guiHtmlInput("radio", "plang"+itoa(i), itoa(i), attrs) + "<label for='plang" + itoa(i) + "'>" + lang + "</label>"
	}
	if wmax := 320; maxwidth > wmax {
		zoomdiv = float64(maxwidth) / float64(wmax)
		zoom = int(100.0 / zoomdiv)
	}
	if rfv := rVal("main_focus_id"); rfv != "" && rfv[0] == 'p' && strings.HasSuffix(rfv, "save") {
		shouldSaveMeta = true
	}
	sv.meta.PanelsTree.iter(func(panel *ImgPanel) {
		rect, pid := panel.Rect, "p"+itoa(pidx)
		w, h := rect.Max.X-rect.Min.X, rect.Max.Y-rect.Min.Y
		cfgdisplay := "none"
		if shouldSaveMeta {
			panel.Areas = nil
			if rVal("main_focus_id") == pid+"save" {
				cfgdisplay = "block"
			}
			for i := 0; i < App.Proj.MaxImagePanelTextAreas; i++ {
				area := ImgPanelArea{Data: A{}}
				for _, lang := range App.Proj.Langs {
					tid := pid + "t" + itoa(i) + lang
					if tval := rVal(tid); tval != "" {
						area.Data[lang] = tval
					}
				}

				trx, trw := rVal(pid+"t"+itoa(i)+"rx"), rVal(pid+"t"+itoa(i)+"rw")
				try, trh := rVal(pid+"t"+itoa(i)+"ry"), rVal(pid+"t"+itoa(i)+"rh")
				if rx0, err := strconv.ParseUint(trx, 0, 64); err == nil {
					if ry0, err := strconv.ParseUint(try, 0, 64); err == nil {
						if rw, err := strconv.ParseUint(trw, 0, 64); err == nil {
							rx1 := rw + rx0
							if rh, err := strconv.ParseUint(trh, 0, 64); err == nil {
								ry1 := rh + ry0
								area.Rect = image.Rect(int(rx0), int(ry0), int(rx1), int(ry1))
								if !area.Rect.Empty() {
									panel.Areas = append(panel.Areas, area)
								}
							}
						}
					}
				}
			}
		}
		langs := []string{}
		for _, lang := range App.Proj.Langs {
			langs = append(langs, lang)
		}
		jsrefr := "refreshPanelRects(" + itoa(pidx) + ", " + itoa(panel.Rect.Min.X) + ", " + itoa(panel.Rect.Min.Y) + ", " + itoa(App.Proj.MaxImagePanelTextAreas) + ", [\"" + strings.Join(langs, "\", \"") + "\"], " + strconv.FormatFloat(px1cm, 'f', 8, 64) + ");"
		btnhtml := guiHtmlButton(pid+"save", "Save changes (all panels)", A{"onclick": "doPostBack(\"" + pid + "save\")"})

		s += "<hr/><h4><u>Panel #" + itoa(pidx+1) + "</u>: " + itoa(len(panel.Areas)) + " text rect(s)" + "</h4><div>Panel coords: " + rect.String() + "</div>"

		s += "<table><tr><td>"
		s += "<div class='panel' style='zoom: " + itoa(zoom) + "%;' onclick='onPanelClick(\"" + pid + "\")'>"
		style := `width: ` + itoa(w) + `px; height: ` + itoa(h) + `px;`
		s += "<div style='position:relative; " + style + "'>"
		style += `background-image: url("/` + sv.meta.bwSmallFilePath + `");`
		style += `background-size: ` + itoa(sv.meta.PanelsTree.Rect.Max.X-sv.meta.PanelsTree.Rect.Min.X) + `px ` + itoa(sv.meta.PanelsTree.Rect.Max.Y-sv.meta.PanelsTree.Rect.Min.Y) + `px;`
		style += `background-position: -` + itoa(rect.Min.X) + `px -` + itoa(rect.Min.Y) + `px;`
		s += "<div class='panelpic' onauxclick='onPanelAuxClick(event, " + itoa(pidx) + ", " + itoa(panel.Rect.Min.X) + ", " + itoa(panel.Rect.Min.Y) + ", " + itoa(App.Proj.MaxImagePanelTextAreas) + ", [\"" + strings.Join(langs, "\", \"") + "\"], " + strconv.FormatFloat(zoomdiv, 'f', 8, 64) + ")' style='" + style + "'></div><span id='" + pid + "rects'></span>"
		s += "</div></div></td><td>"

		s += "<div class='panelcfg' id='" + pid + "cfg' style='display:" + cfgdisplay + ";'>"
		s += btnhtml + "<hr/>"
		for i := 0; i < App.Proj.MaxImagePanelTextAreas; i++ {
			area := ImgPanelArea{Data: A{}}
			if len(panel.Areas) > i {
				area = panel.Areas[i]
			}
			for _, lang := range App.Proj.Langs {
				s += "<div>" + guiHtmlInput("textarea", pid+"t"+itoa(i)+lang, area.Data[lang], A{
					"placeholder": lang,
					"onfocus":     jsrefr, "onblur": jsrefr, "onchange": jsrefr, "onkeydown": jsrefr, "onkeyup": jsrefr, "onkeypress": jsrefr,
					"style": strings.Join(App.Proj.Gen.PanelSvgText.Css[""], ";"),
					"class": "panelcfgtext col" + itoa(i%8)}) + "</div><div>"
			}
			s += "X,Y:"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rx", itoa(area.Rect.Min.X), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.X), "max": itoa(panel.Rect.Max.X)})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"ry", itoa(area.Rect.Min.Y), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.Y), "max": itoa(panel.Rect.Max.Y)})
			s += "W,H:"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rw", itoa(area.Rect.Max.X-area.Rect.Min.X), A{"onchange": jsrefr, "class": "panelcfgrect", "min": "1", "max": itoa(panel.Rect.Max.X - panel.Rect.Min.X)})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rh", itoa(area.Rect.Max.Y-area.Rect.Min.Y), A{"onchange": jsrefr, "class": "panelcfgrect", "min": "1", "max": itoa(panel.Rect.Max.Y - panel.Rect.Min.Y)})
			s += "</div>"
		}
		s += "<hr/>" + btnhtml
		s += "</div>"
		s += "</td></tr></table>"
		s += "<script language='javascript' type='text/javascript'>" + jsrefr + "</script>"
		pidx++
	})
	return
}
