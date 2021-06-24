package main

import (
	"image"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var trim = strings.TrimSpace

func fV(r *http.Request) func(string) string {
	var wthDisAintWindoze = strings.NewReplacer("\r\n", "\n")
	return func(k string) string { return wthDisAintWindoze.Replace(r.FormValue(k)) }
}

func guiMain(r *http.Request, notice string) []byte {
	fv, s := fV(r), "<!DOCTYPE html><html><head><link rel='stylesheet' type='text/css' href='/main.css'/><style type='text/css'>"
	for csssel, csslines := range App.Proj.Gen.PanelSvgText.Css {
		if csssel == "" {
			csssel = "div.panel .panelrect svg text"
		}
		if csslines != nil {
			s += csssel + "{" + strings.Join(csslines, ";") + "}"
		}
	}
	s += "</style><script type='text/javascript' language='javascript'>const svgTxtPerLineDyCmA4 = " + strconv.FormatFloat(App.Proj.Gen.PanelSvgText.PerLineDyCmA4, 'f', 8, 64) + ", svgTxtFontSizeCmA4 = " + strconv.FormatFloat(App.Proj.Gen.PanelSvgText.FontSizeCmA4, 'f', 8, 64) + ";</script><script src='/main.js' type='text/javascript' language='javascript'></script>"
	s += "</head><body><form method='POST' action='/' id='main_form' novalidate='novalidate'>" + guiHtmlInput("hidden", "main_focus_id", fv("main_focus_id"), nil)
	if notice != "" {
		s += "<div class='notice'>" + hEsc(notice) + "</div>"
	}
	if scanJobNotice != "" && fv("scannow") == "" {
		s += "<div class='notice'>Most recent scan job: <b>" + hEsc(scanJobNotice) + "</b> (details if any in stdio)</div>"
		scanJobNotice = ""
	}

	var numseries, numchapters, numsheets int
	for _, series := range App.Proj.Series {
		numseries++
		for _, chapter := range series.Chapters {
			numchapters, numsheets = numchapters+1, numsheets+len(chapter.sheets)
		}
	}

	App.Gui.State.Sel.Series, _ = guiGetFormSel(fv("series"), &App.Proj).(*Series)
	s += guiHtmlList("series", "("+itoa(numseries)+" series, "+itoa(numchapters)+" chapters, "+itoa(numsheets)+" sheets)", false, len(App.Proj.Series), func(i int) (string, string, bool) {
		return App.Proj.Series[i].Name, App.Proj.Series[i].Title["en"] + " (" + itoa(len(App.Proj.Series[i].Chapters)) + ")", App.Gui.State.Sel.Series != nil && App.Proj.Series[i].Name == App.Gui.State.Sel.Series.Name
	})

	shouldsavemeta, havefullgui := false, false
	if series := App.Gui.State.Sel.Series; series != nil {
		App.Gui.State.Sel.Chapter, _ = guiGetFormSel(fv("chapter"), series).(*Chapter)
		s += guiHtmlList("chapter", "("+itoa(len(series.Chapters))+" chapter/s)", false, len(series.Chapters), func(i int) (string, string, bool) {
			chapter := series.Chapters[i]
			return chapter.Name, chapter.Title["en"] + " (" + itoa(len(chapter.sheets)) + ")", App.Gui.State.Sel.Chapter != nil && App.Gui.State.Sel.Chapter.Name == chapter.Name
		})
		if chapter := App.Gui.State.Sel.Chapter; chapter != nil {
			App.Gui.State.Sel.Sheet, _ = guiGetFormSel(fv("sheet"), chapter).(*Sheet)
			numpages, pgnr := 1, 0
			if chapter.SheetsPerPage != 0 {
				numpages = len(chapter.sheets) / chapter.SheetsPerPage
			}
			s += guiHtmlList("sheet", "("+itoa(len(chapter.sheets))+" sheet/s, "+itoa(numpages)+" page/s)", false, len(chapter.sheets), func(i int) (string, string, bool) {
				sheet := chapter.sheets[i]
				if chapter.SheetsPerPage == 0 {
					pgnr = 1
				} else if (i % chapter.SheetsPerPage) == 0 {
					pgnr++
				}
				return sheet.name, "p" + itoa(pgnr) + ": " + sheet.name + " (" + itoa(len(sheet.versions)) + ")", App.Gui.State.Sel.Sheet != nil && App.Gui.State.Sel.Sheet.name == sheet.name
			})
			if sheet := App.Gui.State.Sel.Sheet; sheet == nil {
				havefullgui = true
				s += "<hr/><div id='uipane'>" + guiSheetScan(series, chapter, fv) + "</div>"
			} else if len(sheet.versions) > 0 {
				App.Gui.State.Sel.Ver, _ = guiGetFormSel(fv("sheetver"), sheet).(*SheetVer)
				s += guiHtmlList("sheetver", "", false, len(sheet.versions), func(i int) (string, string, bool) {
					sheetver := sheet.versions[i]
					return sheetver.fileName, sheetver.name, App.Gui.State.Sel.Ver != nil && App.Gui.State.Sel.Ver.fileName == sheetver.fileName
				})
				if App.Gui.State.Sel.Ver == nil {
					App.Gui.State.Sel.Ver = sheet.versions[0]
				}
				if sheetver := App.Gui.State.Sel.Ver; sheetver != nil {
					havefullgui = true
					s += "<hr/><div id='uipane'>" + guiSheetEdit(sheetver, fv, &shouldsavemeta) + "</div>"
				}
			}
		}
	}
	if !havefullgui {
		s += "<hr/><div id='uipane'>" + guiStartView() + "</div>"
	}
	if shouldsavemeta {
		App.Proj.save()
	}

	s += "<hr/>" + guiHtmlListFrom("main_action", "(Project Actions)", true, AppMainActions)

	s += "</form></body>"
	if rfv := fv("main_focus_id"); rfv != "" && rfv != "main_action" && notice == "" {
		s += "<script language='javascript' type='text/javascript'>try { document.getElementById(\"" + rfv + "\").focus(); } catch (e) {alert(e);}</script></html>"
	}
	return []byte(s)
}

func guiStartView() (s string) {
	for _, series := range App.Proj.Series {
		if App.Gui.State.Sel.Series == nil || App.Gui.State.Sel.Series == series {
			for _, chapter := range series.Chapters {
				if id := "chk" + itoa(int(time.Now().UnixNano())); App.Gui.State.Sel.Series == nil || App.Gui.State.Sel.Chapter == nil || App.Gui.State.Sel.Chapter == chapter {
					numpages := 1
					if chapter.SheetsPerPage != 0 {
						numpages = len(chapter.sheets) / chapter.SheetsPerPage
					}
					s += "<div><input class='collchk' id='" + id + "' type='checkbox' checked='checked'/><h3><label for='" + id + "'>" + series.Title["en"] + "&nbsp;&nbsp;&mdash;&nbsp;&nbsp;" + chapter.Title["en"] + "&nbsp;&nbsp;&mdash;&nbsp;&nbsp;(" + itoa(len(chapter.sheets)) + " sheet/s, " + itoa(numpages) + " page/s)</label></h3>"
					s += "<div class='collchk'><table width='99%'>"
					pgnr := 0
					for i, sheet := range chapter.sheets {
						if chapter.SheetsPerPage == 0 {
							pgnr = 1
						} else if (i % chapter.SheetsPerPage) == 0 {
							pgnr++
						}
						for _, sv := range sheet.versions {
							s += "<tr><td valign='top'>"
							numpanels, numpanelareas := sv.panelCount()
							a := "<a href='./?series=" + url.QueryEscape(series.Name) + "&chapter=" + url.QueryEscape(chapter.Name) + "&sheet=" + url.QueryEscape(sheet.name) + "&sheetver=" + url.QueryEscape(sv.fileName) + "'>"
							if sv.data != nil {
								s += a + "<img title='" + hEsc(sheet.name+"_"+sv.name) + "' src='./" + sv.data.bwSmallFilePath + "' style='width: 11em;'/></a>"
							}
							s += "</td><td width='98%' valign='top' align='left'><h4 style='margin-top: 0;'>p" + itoa(pgnr) + "&nbsp;&nbsp;&mdash;&nbsp;&nbsp;" + a + hEsc(sheet.name+"_"+sv.name) + "</a>"
							if numpanels > 0 {
								s += "<small>&nbsp;&nbsp;&mdash;&nbsp;&nbsp;<b>" + itoa(numpanelareas) + " </b> text-rect(s) in " + itoa(numpanels) + " panel(s)"
								if numpanelareas > 0 {
									for langid, percent := range sv.percentTranslated() {
										s += "&nbsp;(<b>" + langid + "</b>: " + strconv.FormatFloat(percent, 'f', 1, 64) + "%)"
									}
								}
								s += "</small>"
							}
							if sv.data != nil {
								if sv.data.PanelsTree != nil {
									s += "<small>&nbsp;&mdash;&nbsp;&nbsp;<b>" + itoa(sv.data.PanelsTree.Rect.Max.X) + "&times;" + itoa(sv.data.PanelsTree.Rect.Max.Y) + "</b>px (" + itoa(int(sv.Px1Cm())) + "px/cm)</small>"
								}
								s += "<small>&nbsp;&nbsp;&mdash;&nbsp;&nbsp;from <b title='" + hEsc(time.Unix(0, sv.data.DateTimeUnixNano).Format(time.RFC1123)) + "'>" + time.Unix(0, sv.data.DateTimeUnixNano).Format("02 Jan 2006") + "</b></small>"
							}
							s += "</h4>" + guiHtmlGrayDistrs(sv.grayDistrs()) + "</td></tr>"
						}
					}
					s += "</table><hr/></div>"
					s += "</div>"
				}
			}
		}
	}
	return
}

func guiSheetScan(series *Series, chapter *Chapter, fv func(string) string) (s string) {
	if fv("scannow") != "" && scanJob == nil {
		sj := ScanJob{
			Id:     strconv.FormatInt(time.Now().UnixNano(), 36),
			Series: series, Chapter: chapter, Opts: map[string]string{},
			SheetName: trim(fv("sheetname")), SheetVerName: trim(fv("sheetvername")),
		}
		sj.PnmFileName, sj.PngFileName = ".csg/tmp/"+sj.Id+".pnm", "sheets/"+series.Name+"/"+chapter.Name+"/sheets/"+sj.SheetName+"_"+sj.SheetVerName+".png"
		for _, sd := range scanDevices {
			if sd.Ident == fv("scandev") {
				sj.Dev = sd
				break
			}
		}
		if sj.Dev != nil {
			for _, opt := range sj.Dev.Options {
				if opt.Inactive {
					continue
				}
				if formval := trim(fv(sj.Dev.Ident + "_opt_" + opt.Name)); formval != "" {
					sj.Opts[opt.Name] = formval
				}
			}
			scanJob, scanJobNotice = &sj, ""
			go scanJobDo()
		}
	}
	if len(scanDevices) == 0 {
		return "<div>(Scanner device detection still ongoing)</div>"
	} else if scanJob != nil {
		s = "<div>Scan job in <a href='/?series=" + url.QueryEscape(scanJob.Series.Name) + "&chapter=" + url.QueryEscape(scanJob.Chapter.Name) + "&t=" + itoa(int(time.Now().UnixNano())) + "'>progress</a> on device <b>" + scanJob.Dev.String() + "</b>:<br/>"
		if scanJob.Series == series && scanJob.Chapter == chapter {
			return s + "first into <code>" + scanJob.PnmFileName + "</code>, then <code>" + hEsc(scanJob.PngFileName) + "</code></div>"
		}
		return s + "for other chapter <code>" + scanJob.Series.Name + "/" + scanJob.Chapter.Name + "</code>)</div>"
	}

	s += "<h3>New Sheet Version Scan</h3>"
	s += guiHtmlInput("text", "sheetname", "", A{"placeholder": "Sheet Name"})
	s += guiHtmlInput("text", "sheetvername", "", A{"placeholder": "Sheet Version Name"})
	s += "<h3>Scanner To Use:</h3>"

	s += "<div><select name='scandev' id='scandev' onchange='toggleScanOptsPane(this.options[this.selectedIndex].value)'>"
	for i, sd := range scanDevices {
		htmlsel := " selected='selected'"
		if i != len(scanDevices)-1 {
			htmlsel = ""
		}
		s += "<option value='" + sd.Ident + "'" + htmlsel + ">" + hEsc(sd.String()) + "</option>"
	}
	s += "</select></div><div class='scandevoptsbox'>"
	for i, sd := range scanDevices {
		cssdisplay := "block"
		if i != len(scanDevices)-1 {
			cssdisplay = "none"
		}
		s += "<div class='scandevopts' id='scandevopts_" + sd.Ident + "' style='display: " + cssdisplay + "'>"
		defvals, dontshow := saneDevDefaults[sd.Ident], append(saneDevDontShow[sd.Ident], saneDevDontShow[""]...)
		if defvals == nil {
			defvals = map[string]string{}
		}
		var cat string
		for _, opt := range sd.Options {
			var hide bool
			for _, ds := range dontshow {
				if hide = (ds == opt.Name); (!hide) && (ds[0] == '*' || ds[len(ds)-1] == '*') {
					if ds[0] == '*' && ds[len(ds)-1] == '*' {
						hide = strings.Contains(opt.Name, ds[1:len(ds)-2])
					} else if ds[0] == '*' {
						hide = strings.HasSuffix(opt.Name, ds[1:])
					} else {
						hide = strings.HasPrefix(opt.Name, ds[:len(ds)-2])
					}
				}
				if hide {
					break
				}
			}
			if hide {
				continue
			}
			if opt.Category != cat && opt.Category != "" {
				cat = opt.Category
				s += "<h3><i>" + hEsc(cat) + "</i></h3>"
			}
			htmlid := sd.Ident + "_opt_" + opt.Name
			s += "<div class='scandevopt'><div class='scandevoptheader'>"
			defval := defvals[opt.Name]
			if defval == "" {
				defval = saneDevDefaults[""][opt.Name]
			}
			httitle := hEsc(strings.Replace(strings.Replace(strings.Join(opt.Description, "\n"), "\"", "`", -1), "'", "`", -1))
			attrs := A{"title": httitle}
			if opt.Inactive {
				attrs["readonly"], attrs["disabled"] = "readonly", "disabled"
			}
			if !opt.IsToggle {
				s += guiHtmlInput("text", htmlid, defval, attrs)
			} else {
				if defval == "yes" && !opt.Inactive {
					attrs["checked"] = "checked"
				}
				s += guiHtmlInput("checkbox", htmlid, "yes", attrs)
			}
			ht := "b"
			if opt.Inactive {
				ht = "del"
			}
			s += "&nbsp;<label for='" + htmlid + "' title='" + httitle + "'><" + ht + ">" + opt.Name + "</" + ht + "></label>"
			if fi := opt.FormatInfo; fi != "" && !opt.IsToggle {
				var misc string
				if idx := strings.LastIndexByte(fi, '['); fi[len(fi)-1] == ']' && idx > 0 {
					misc, fi = fi[idx:], fi[:idx]
				}
				s += " &mdash; <span title='" + hEsc(misc) + "'>" + hEsc(strings.Replace(fi, "|", " | ", -1)) + "</span>"
			}
			s += "</div><div class='scandevoptdesc' id='scandevoptdesc_" + sd.Ident + "_" + opt.Name + "'>"
			for _, desc := range opt.Description {
				s += "<div>" + hEsc(desc) + "</div>"
			}
			s += "</div></div><hr/>"
		}
		s += "</div>"
	}
	s += "<input type='hidden' name='scannow' id='scannow' value=''/><button type='button' id='scanbtn' onclick='kickOffScanJob()'>Begin Scan Now</button>"
	s += "</div>"
	return
}

func guiSheetEdit(sv *SheetVer, fv func(string) string, shouldSaveMeta *bool) (s string) {
	sv.ensurePrep(false, false)
	numpanels, maxpanelwidth, bwsrc := 0, 0, fv("srcpx")
	sv.data.PanelsTree.iter(func(panel *ImgPanel) {
		numpanels++
		if w := panel.Rect.Max.X - panel.Rect.Min.X; w > maxpanelwidth {
			maxpanelwidth = w
		}
	})
	if bwsrc != sv.data.bwSmallFilePath && bwsrc != sv.data.bwFilePath {
		bwsrc = sv.data.bwSmallFilePath
	}
	s = "<h3>Full Sheet:&nbsp;"
	if sw, bw := sv.data.PanelsTree.Rect.Max.X, int(App.Proj.BwSmallWidth); sw > bw {
		s += guiHtmlList("srcpx", "", true, 2, func(i int) (string, string, bool) {
			if i == 1 {
				return sv.data.bwFilePath, itoa(sw) + "px", bwsrc == sv.data.bwFilePath
			}
			return sv.data.bwSmallFilePath, itoa(bw) + "px", true
		})
	}
	s += "&nbsp;&mdash; jump to panel:"
	for i := 0; i < numpanels; i++ {
		s += "&nbsp;&nbsp;<a href='#pa" + sv.Id() + itoa(i) + "'>" + itoa(i+1) + "</a>"
	}
	s += "</h3>"
	graydistrs := sv.grayDistrs()
	s += guiHtmlGrayDistrs(graydistrs)
	s += "<div>B&amp;W version at black-threshold of <b>&lt;" + itoa(int(App.Proj.BwThreshold)) + "</b>:</div>"
	s += "<div class='fullsheet'>" + guiHtmlImg("/"+bwsrc, nil) + "</div>"
	s += "<div>View other B&amp;W thresholds: <input type='text' id='previewbwt' onchange='addBwtPreviewLinks(\"" + sv.fileName + "\");'/><div id='previewbwtlinks'></div></div>"

	var panelstree func(*ImgPanel) string
	pidx := 0
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
			s += "<ul><li><div><b><a href='#pa" + sv.Id() + itoa(pidx) + "'>Panel #" + itoa(pidx+1) + "</a></b>: " + panel.Rect.String() + "</div></li></ul>"
			pidx++
		}
		return
	}
	s += "<h3>Sheet Panels Structure:</h3><ul><li>Sheet coords:" + sv.data.PanelsTree.Rect.String() + panelstree(sv.data.PanelsTree) + "</li></ul><hr/>"
	zoom, zoomdiv := 100, 1.0
	s += "<h3>All " + itoa(numpanels) + " panel(s):"
	for i := 0; i < numpanels; i++ {
		s += "&nbsp;&nbsp;<a href='#pa" + sv.Id() + itoa(i) + "'>" + itoa(i+1) + "</a>"
	}
	for i, lang := range App.Proj.Langs {
		attrs := A{"name": "plang", "onclick": "refreshAllPanelRects(" + itoa(numpanels) + "," + itoa(i) + ",\"" + lang + "\");"}
		if i == 0 {
			attrs["checked"] = "checked"
		}
		s += "&nbsp;&nbsp;" + guiHtmlInput("radio", "plang"+itoa(i), itoa(i), attrs) + "<label for='plang" + itoa(i) + "'>" + lang + "</label>"
	}
	s += "</h3><div>"
	importlist := map[string]string{}
	for sheetfilename, panelsareas := range App.Proj.data.sheetVerPanelAreas {
		if sheetfilename != sv.fileName {
			var numareas int
			for _, panelareas := range panelsareas {
				numareas += len(panelareas)
			}
			importlist[sheetfilename] = sheetfilename + " (" + itoa(numareas) + " text area(s) in " + itoa(len(panelsareas)) + " panel(s))"
		}
	}
	s += "</div><div>" + guiHtmlListFrom("importpaneltexts", "(Import panel text areas from another sheet where panel indices match)", true, importlist) + "</div>"
	var importfrom string
	if fv("main_focus_id") == "importpaneltexts" {
		*shouldSaveMeta, importfrom = true, fv("importpaneltexts")
	}
	if wmax := 320; maxpanelwidth > wmax {
		zoomdiv = float64(maxpanelwidth) / float64(wmax)
		zoom = int(100.0 / zoomdiv)
	}
	if rfv := fv("main_focus_id"); rfv != "" && rfv[0] == 'p' && strings.HasSuffix(rfv, "save") {
		*shouldSaveMeta = true
	}
	if *shouldSaveMeta {
		App.Proj.data.sheetVerPanelAreas[sv.fileName] = nil
	}
	pidx = 0
	sv.data.PanelsTree.iter(func(panel *ImgPanel) {
		rect, pid := panel.Rect, "p"+itoa(pidx)
		w, h := rect.Max.X-rect.Min.X, rect.Max.Y-rect.Min.Y
		cfgdisplay := "none"
		if *shouldSaveMeta {
			App.Proj.data.sheetVerPanelAreas[sv.fileName] = append(App.Proj.data.sheetVerPanelAreas[sv.fileName], []ImgPanelArea{})
			if fv("main_focus_id") == pid+"save" {
				cfgdisplay = "block"
			}
			for i := 0; i < App.Proj.MaxImagePanelTextAreas; i++ {
				area := ImgPanelArea{Data: A{}}
				for _, lang := range App.Proj.Langs {
					tid := pid + "t" + itoa(i) + lang
					if tval := fv(tid); trim(tval) != "" {
						area.Data[lang] = strings.TrimRight(tval, "\n \t\r\v\b")
					}
				}

				area.SvgTextTransformAttr = trim(fv(pid + "t" + itoa(i) + "_transform"))
				area.SvgTextTspanStyleAttr = trim(fv(pid + "t" + itoa(i) + "_style"))
				trx, trw := trim(fv(pid+"t"+itoa(i)+"rx")), trim(fv(pid+"t"+itoa(i)+"rw"))
				try, trh := trim(fv(pid+"t"+itoa(i)+"ry")), trim(fv(pid+"t"+itoa(i)+"rh"))
				rpx, rpy := trim(fv(pid+"t"+itoa(i)+"rpx")), trim(fv(pid+"t"+itoa(i)+"rpy"))
				if rx0, err := strconv.ParseUint(trx, 0, 64); err == nil {
					if ry0, err := strconv.ParseUint(try, 0, 64); err == nil {
						if rw, err := strconv.ParseUint(trw, 0, 64); err == nil {
							rx1 := rw + rx0
							if rh, err := strconv.ParseUint(trh, 0, 64); err == nil {
								ry1 := rh + ry0
								area.Rect = image.Rect(int(rx0), int(ry0), int(rx1), int(ry1))
								if !area.Rect.Empty() {
									if rpx != "" && rpy != "" {
										if rpx, err := strconv.ParseInt(rpx, 0, 64); err == nil {
											if rpy, err := strconv.ParseInt(rpy, 0, 64); err == nil {
												area.PointTo = &image.Point{int(rpx), int(rpy)}
											}
										}
									}
									App.Proj.data.sheetVerPanelAreas[sv.fileName][pidx] = append(App.Proj.data.sheetVerPanelAreas[sv.fileName][pidx], area)
								}
							}
						}
					}
				}
			}
			if panelsareas := App.Proj.data.sheetVerPanelAreas[importfrom]; len(panelsareas) > pidx {
				for _, area := range panelsareas[pidx] {
					if !area.Rect.Empty() {
						App.Proj.data.sheetVerPanelAreas[sv.fileName][pidx] = append(App.Proj.data.sheetVerPanelAreas[sv.fileName][pidx], area)
					}
				}
			}
		}
		langs := []string{}
		for _, lang := range App.Proj.Langs {
			langs = append(langs, lang)
		}
		jsrefr := "refreshPanelRects(" + itoa(pidx) + ", " + itoa(panel.Rect.Min.X) + ", " + itoa(panel.Rect.Min.Y) + ", " + itoa(panel.Rect.Max.X-panel.Rect.Min.X) + ", " + itoa(panel.Rect.Max.Y-panel.Rect.Min.Y) + ", " + itoa(App.Proj.MaxImagePanelTextAreas) + ", [\"" + strings.Join(langs, "\", \"") + "\"], " + strconv.FormatFloat(sv.Px1Cm(), 'f', 8, 64) + ", '" + App.Proj.Gen.PanelSvgText.ClsBoxPoly + "', " + strconv.FormatFloat(App.Proj.Gen.PanelSvgText.BoxPolyStrokeWidthCm, 'f', 8, 64) + ");"
		btnhtml := guiHtmlButton(pid+"save", "Save changes (all panels)", A{"onclick": "doPostBack(\"" + pid + "save\")"})

		s += "<hr/><h4 id='pa" + sv.Id() + itoa(pidx) + "'><u>Panel #" + itoa(pidx+1) + "</u>: " + itoa(len(sv.panelAreas(pidx))) + " text rect(s)" + "</h4><div>Panel coords: " + rect.String() + "</div>"

		s += "<table><tr><td>"
		s += "<div class='panel' style='zoom: " + itoa(zoom) + "%;' onclick='onPanelClick(\"" + pid + "\")'>"
		style := `width: ` + itoa(w) + `px; height: ` + itoa(h) + `px;`
		s += "<div style='position:relative; " + style + "' onauxclick='onPanelAuxClick(event, " + itoa(pidx) + ", " + itoa(panel.Rect.Min.X) + ", " + itoa(panel.Rect.Min.Y) + ", " + itoa(App.Proj.MaxImagePanelTextAreas) + ", [\"" + strings.Join(langs, "\", \"") + "\"], " + strconv.FormatFloat(zoomdiv, 'f', 8, 64) + ")' onmousemove='this.title=parseInt(" + itoa(panel.Rect.Min.X) + "+event.offsetX*" + strconv.FormatFloat(zoomdiv, 'f', 8, 64) + ")+\",\"+parseInt(" + itoa(panel.Rect.Min.Y) + "+event.offsetY*" + strconv.FormatFloat(zoomdiv, 'f', 8, 64) + ")'>"
		style += `background-image: url("/` + bwsrc + `");`
		style += `background-size: ` + itoa(sv.data.PanelsTree.Rect.Max.X-sv.data.PanelsTree.Rect.Min.X) + `px ` + itoa(sv.data.PanelsTree.Rect.Max.Y-sv.data.PanelsTree.Rect.Min.Y) + `px;`
		style += `background-position: -` + itoa(rect.Min.X) + `px -` + itoa(rect.Min.Y) + `px;`
		s += "<div class='panelpic' style='" + style + "'></div><span id='" + pid + "rects'></span>"
		s += "</div></div></td><td>"

		s += "<div class='panelcfg' id='" + pid + "cfg' style='display:" + cfgdisplay + ";'>"
		s += btnhtml
		for i := 0; i < App.Proj.MaxImagePanelTextAreas; i++ {
			area := ImgPanelArea{Data: A{}}
			if panelareas := sv.panelAreas(pidx); len(panelareas) > i {
				area = panelareas[i]
			}
			s += "<hr/>"
			for _, lang := range App.Proj.Langs {
				s += "<div>" + guiHtmlInput("textarea", pid+"t"+itoa(i)+lang, area.Data[lang], A{
					"placeholder": lang,
					"onfocus":     jsrefr, "onblur": jsrefr, "onchange": jsrefr, "onkeydown": jsrefr, "onkeyup": jsrefr, "onkeypress": jsrefr,
					"style": strings.Join(App.Proj.Gen.PanelSvgText.Css[""], ";"),
					"class": "panelcfgtext col" + itoa(i%8)}) + "</div><div style='text-align: center; white-space: nowrap;'>"
			}

			s += "xy"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rx", itoa(area.Rect.Min.X), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.X), "max": itoa(panel.Rect.Max.X)})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"ry", itoa(area.Rect.Min.Y), A{"onchange": jsrefr, "class": "panelcfgrect", "min": itoa(panel.Rect.Min.Y), "max": itoa(panel.Rect.Max.Y)})
			s += "wh"
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rw", itoa(area.Rect.Max.X-area.Rect.Min.X), A{"onchange": jsrefr, "class": "panelcfgrect", "min": "1", "max": itoa(panel.Rect.Max.X - panel.Rect.Min.X)})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rh", itoa(area.Rect.Max.Y-area.Rect.Min.Y), A{"onchange": jsrefr, "class": "panelcfgrect", "min": "1", "max": itoa(panel.Rect.Max.Y - panel.Rect.Min.Y)})
			s += "p"
			px, py := "", ""
			if area.PointTo != nil {
				px, py = itoa(area.PointTo.X), itoa(area.PointTo.Y)
			}
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rpx", px, A{"onchange": jsrefr, "class": "panelcfgrect", "min": "0", "max": itoa(panel.Rect.Max.X)})
			s += guiHtmlInput("number", pid+"t"+itoa(i)+"rpy", py, A{"onchange": jsrefr, "class": "panelcfgrect", "min": "0", "max": itoa(panel.Rect.Max.Y)})

			s += "</div><div style='text-align: center;'>" + guiHtmlInput("textarea", pid+"t"+itoa(i)+"_transform", area.SvgTextTransformAttr, A{
				"class": "panelcfgtextattr", "title": "translate(x [,y])\tscale(x [,y])\trotate(a [,oX] [,oY])\tmatrix(a,b,c,d,e,f)\tskewX(x)\tskewY(y)",
				"onfocus": jsrefr, "onblur": jsrefr, "onchange": jsrefr, "onkeydown": jsrefr, "onkeyup": jsrefr, "onkeypress": jsrefr,
			}) + guiHtmlInput("textarea", pid+"t"+itoa(i)+"_style", area.SvgTextTspanStyleAttr, A{
				"class": "panelcfgtextattr", "title": "style attr for the SVG <tspan> element",
				"onfocus": jsrefr, "onblur": jsrefr, "onchange": jsrefr, "onkeydown": jsrefr, "onkeyup": jsrefr, "onkeypress": jsrefr,
			}) + "</div>"
		}
		s += "<hr/>" + btnhtml
		s += "</div>"
		s += "</td></tr></table>"
		s += "<script language='javascript' type='text/javascript'>" + jsrefr + "</script>"

		pidx++
	})
	return
}
