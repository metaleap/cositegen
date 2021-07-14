package main

import (
	"image"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func fV(r *http.Request) func(string) string {
	var wthDisAintWindoze = strings.NewReplacer("\r\n", "\n")
	return func(k string) string { return wthDisAintWindoze.Replace(r.FormValue(k)) }
}

func guiMain(r *http.Request, notice string) []byte {
	fv, dirpref, s := fV(r), "", "<!DOCTYPE html><html lang='en'><head><meta charset='utf-8'><link rel='stylesheet' type='text/css' href='/main.css'/><style type='text/css'>"
	for k := range App.Proj.Gen.PanelSvgText.AppendToFiles {
		if idx := strings.LastIndexByte(k, '/'); idx > 0 {
			dirpref = k[:idx]
			break
		}
	}
	for csssel, csslines := range App.Proj.Gen.PanelSvgText.Css {
		if csssel == "" {
			csssel = "div.panel .panelrect svg text"
		}
		if csslines != nil {
			s += csssel + "{" + strings.Replace(strings.Join(csslines, ";"), "./", dirpref+"/", -1) + "}"
		}
	}
	s += "</style><script type='text/javascript' language='javascript'>const $ = window, svgTxtPerLineDyCmA4 = " + ftoa(App.Proj.Gen.PanelSvgText.PerLineDyCmA4, 8) + ", svgTxtFontSizeCmA4 = " + ftoa(App.Proj.Gen.PanelSvgText.FontSizeCmA4, 8) + ", numImagePanelTextAreas = " + itoa(App.Proj.MaxImagePanelTextAreas) + ";</script><script src='/main.js' type='text/javascript' language='javascript'></script>"
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
		return App.Proj.Series[i].Name, App.Proj.Series[i].Name + " (" + itoa(len(App.Proj.Series[i].Chapters)) + ")", App.Gui.State.Sel.Series != nil && App.Proj.Series[i].Name == App.Gui.State.Sel.Series.Name
	})

	shouldsavemeta, havefullgui := false, false
	if series := App.Gui.State.Sel.Series; series != nil {
		App.Gui.State.Sel.Chapter, _ = guiGetFormSel(fv("chapter"), series).(*Chapter)
		s += guiHtmlList("chapter", "("+itoa(len(series.Chapters))+" chapter/s)", false, len(series.Chapters), func(i int) (string, string, bool) {
			chapter := series.Chapters[i]
			return chapter.Name, chapter.Name + " (" + itoa(len(chapter.sheets)) + ")", App.Gui.State.Sel.Chapter != nil && App.Gui.State.Sel.Chapter.Name == chapter.Name
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
				s += "<hr/><div id='uipane'>" + guiSheetScan(chapter, fv) + "</div>"
			} else if len(sheet.versions) > 0 {
				App.Gui.State.Sel.Ver, _ = guiGetFormSel(fv("sheetver"), sheet).(*SheetVer)
				s += guiHtmlList("sheetver", "", false, len(sheet.versions), func(i int) (string, string, bool) {
					sheetver := sheet.versions[i]
					return sheetver.fileName, time.Unix(0, sheetver.dateTimeUnixNano).Format("2006-01-02"), App.Gui.State.Sel.Ver != nil && App.Gui.State.Sel.Ver.fileName == sheetver.fileName
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
		s += "<script language='javascript' type='text/javascript'>try { document.getElementById('" + rfv + "').focus(); } catch (e) {alert(e);}</script></html>"
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
					s += "<div><input class='collchk' id='" + id + "' type='checkbox' checked='checked'/><h3><label for='" + id + "'>" + series.Name + "&nbsp;&nbsp;&horbar;&nbsp;&nbsp;" + chapter.Name + "&nbsp;&nbsp;&horbar;&nbsp;&nbsp;(" + itoa(len(chapter.sheets)) + " sheet/s, " + itoa(numpages) + " page/s)</label></h3>"
					s += "<div class='collchk'><table width='99%'>"
					pgnr, pgprev := 0, 0
					for i, sheet := range chapter.sheets {
						if chapter.SheetsPerPage == 0 {
							pgnr, pgprev = 1, 1
						} else if (i % chapter.SheetsPerPage) == 0 {
							pgnr++
						}
						for svidx, sv := range sheet.versions {
							s += "<tr><td valign='top'>"
							if pgnr != pgprev {
								s += "<hr/>"
							}
							numpanels, numpanelareas := sv.panelCount()
							a := "<a href='./?series=" + url.QueryEscape(series.Name) + "&chapter=" + url.QueryEscape(chapter.Name) + "&sheet=" + url.QueryEscape(sheet.name) + "&sheetver=" + url.QueryEscape(sv.fileName) + "&t=" + itoa(int(time.Now().UnixNano())) + "'>"
							if sv.data != nil {
								if fi := fileStat(sv.data.bwSmallFilePath); fi != nil && fi.Size() > 0 {
									s += a + "<img title='" + hEsc(sheet.name) + "' src='./" + sv.data.bwSmallFilePath + "' style='width: 11em;'/></a>"
								}
							}
							s += "</td><td width='98%' valign='top' align='left'>"
							if pgnr != pgprev {
								s += "<hr/>"
							}
							s += "<h4 style='margin-top: 0;'><span"
							if svidx != 0 {
								s += " style='visibility:hidden'"
							}
							s += ">p" + itoa(pgnr) + "</span>&nbsp;&nbsp;&horbar;&nbsp;&nbsp;" + a + hEsc(sheet.name) + "</a>"
							if numpanels > 0 {
								s += "<small>&nbsp;&nbsp;&horbar;&nbsp;&nbsp;<b>" + itoa(numpanelareas) + " </b> text" + strIf(numpanelareas == 1, "", "s") + " in " + itoa(numpanels) + " panel/s"
								if numpanelareas > 0 {
									for _, langid := range App.Proj.Langs[1:] {
										s += "&nbsp;(<b>" + langid + "</b>: " + ftoa(App.Proj.percentTranslated(langid, nil, nil, sv, -1), 1) + "%)"
									}
								}
								s += "</small>"
							}
							if sv.data != nil && sv.data.PanelsTree != nil {
								s += "<small>&nbsp;&horbar;&nbsp;&nbsp;<b>" + itoa(sv.data.PanelsTree.Rect.Max.X) + "&times;" + itoa(sv.data.PanelsTree.Rect.Max.Y) + "</b>px (" + ftoa(sv.data.PxCm, 1) + "px/cm)</small>"
							}
							s += "<small>&nbsp;&nbsp;&horbar;&nbsp;&nbsp;from <b>" + time.Unix(0, sv.dateTimeUnixNano).Format("02 Jan 2006") + "</b></small>"
							s += "</h4>" + guiHtmlGrayDistrs(sv.grayDistrs()) + "</td></tr>"
						}
						pgprev = pgnr
					}
					s += "</table></div>"
					s += "</div>"
				}
			}
		}
	}
	return
}

func guiSheetScan(chapter *Chapter, fv func(string) string) (s string) {
	series := chapter.parentSeries
	if fv("scannow") != "" && scanJob == nil {
		sheetname, sheetvername, sj := trim(fv("sheetname")), time.Now().Format("20060102"), ScanJob{
			Id:     strconv.FormatInt(time.Now().UnixNano(), 36),
			Series: series, Chapter: chapter, Opts: map[string]string{},
		}
		sj.PnmFileName, sj.PngFileName = "/dev/shm/csg"+sj.Id+".pnm", "scans/"+series.Name+"/"+chapter.Name+"/"+sheetname+"."+sheetvername+".png"
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
	s += guiHtmlInput("text", "sheetname", "", A{"placeholder": "Sheet Name"}) +
		"." + guiHtmlInput("text", "sheetvername", time.Now().Format("20060102"), A{"disabled": "disabled"}) + ".png"
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
				s += " &horbar; <span title='" + hEsc(misc) + "'>" + hEsc(strings.Replace(fi, "|", " | ", -1)) + "</span>"
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
		if w := panel.Rect.Dx(); w > maxpanelwidth {
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
	s += "&nbsp;&horbar; jump to panel:"
	for i := 0; i < numpanels; i++ {
		s += "&nbsp;&nbsp;<a href='#pa" + sv.id + itoa(i) + "'>" + itoa(i+1) + "</a>"
	}
	s += "</h3>"
	graydistrs := sv.grayDistrs()
	s += guiHtmlGrayDistrs(graydistrs)
	s += "<div><select onchange='$.fsimg.style.backgroundImage=this.value;'><option value='none'>Black&amp;White</option>"
	if bgfilename := sv.fileName[:len(sv.fileName)-len(".png")] + ".svg"; fileStat(bgfilename) != nil && fv("srcpx") != sv.data.bwFilePath {
		s += "<option value='url(\"/" + bgfilename + "\")'>Colorized</option>"
	}
	s += "</select> version at black-threshold of <select onchange='$.fsimg.src=this.value;'>"
	for i, bwt := range App.Proj.BwThresholds {
		href := "/" + sv.fileName + "/" + itoa(int(bwt))
		if i == 0 {
			href = "/" + bwsrc
		} else if fv("srcpx") != sv.data.bwFilePath {
			break
		}
		s += "<option value='" + href + "' style='background-image: url(\"" + href + "\");'>&lt; " + itoa(int(bwt)) + " (" + strIf(i == 0, "current", "preview") + ")" + "</option>"
	}
	s += "</select></div>"
	s += "<div id='fullsheet'>" + guiHtmlImg("/"+bwsrc, A{"id": "fsimg", "style": "background-image: none"})
	if len(sv.parentSheet.parentChapter.storyBoardPages) > 0 {
		sw, sh := sv.sizeCm()
		for i, pg := range sv.parentSheet.parentChapter.storyBoardPages {
			doimport := (fv("txtimpsel") == itoa(i)) && (fv("main_focus_id") == "txtimpsel")
			pareas := App.Proj.data.Sv.textRects[sv.id]
			if doimport {
				for np, _ := sv.panelCount(); len(pareas) < np; {
					pareas = append(pareas, []ImgPanelArea{})
				}
			}
			s += "<div id='txtprev" + itoa(i) + "' class='txtprev'>"
			for _, tb := range pg.textBoxes {
				xywh := sv.cmToPx(tb.xywhCm...)
				rect := image.Rect(xywh[0], xywh[1], xywh[0]+xywh[2], xywh[1]+xywh[3])
				_, pidx := sv.panelMostCoveredBy(rect)
				px, py, pw, ph := 100.0/(sw/tb.xywhCm[0]), 100.0/(sh/tb.xywhCm[1]), 100.0/(sw/tb.xywhCm[2]), 100.0/(sh/tb.xywhCm[3])
				fulltext := strings.Join(tb.textSpans, " ")
				s += "<div style='left: " + ftoa(px, 1) + "%; top: " + ftoa(py, 1) + "%; width: " + ftoa(pw, 1) + "%; xheight: " + ftoa(ph, 1) + "%;' class='txtprevbox' title='" + hEsc(fulltext) + "'>"
				s += "<b><i>(P" + itoa(pidx) + ") </i></b> " + fulltext + "</div>"
				if doimport && pidx >= 0 && pidx < len(pareas) {
					pareas[pidx] = append(pareas[pidx], ImgPanelArea{
						Data: map[string]string{App.Proj.Langs[0]: fulltext},
						Rect: rect,
					})
				}
			}
			if doimport {
				*shouldSaveMeta, App.Proj.data.Sv.textRects[sv.id] = true, pareas
			}
			s += "</div>"
		}
	}
	s += "</div><div>Preview other B&amp;W thresholds: <input type='text' id='previewbwt' onchange='addBwtPreviewLinks(\"" + sv.fileName + "\");'/><div id='previewbwtlinks'></div></div><div>"
	if len(sv.parentSheet.parentChapter.storyBoardPages) > 0 {
		s += "<select name='txtimpsel' id='txtimpsel' onchange='txtPrev(this.value);'><option value=''>(Preview story-board text import...)</option>"
		for i, pg := range sv.parentSheet.parentChapter.storyBoardPages {
			s += "<option value='" + itoa(i) + "'>" + hEsc(pg.name) + "</option>"
		}
		s += "</select>&nbsp;&nbsp;<button onclick='txtImp()'>Import Now!</button>&nbsp;&nbsp;&nbsp;&nbsp;"
	}
	s += "</div>"

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
			s += "<ul><li><div><b><a href='#pa" + sv.id + itoa(pidx) + "'>Panel #" + itoa(pidx+1) + "</a></b>: " + panel.Rect.String() + "</div></li></ul>"
			pidx++
		}
		return
	}
	s += "<h3>Sheet Panels Structure:</h3><ul><li>Sheet coords:" + sv.data.PanelsTree.Rect.String() + panelstree(sv.data.PanelsTree) + "</li></ul><hr/>"
	zoom, zoomdiv := 100, 1.0
	s += "<h3>All " + itoa(numpanels) + " panel/s:"
	for i := 0; i < numpanels; i++ {
		s += "&nbsp;&nbsp;<a href='#pa" + sv.id + itoa(i) + "'>" + itoa(i+1) + "</a>"
	}
	s += "</h3><div>"
	importlist := map[string]string{}
	for svid, panels := range App.Proj.data.Sv.textRects {
		if svid != sv.id {
			sheetfilename := App.Proj.data.Sv.IdsToFileNames[svid]
			if sheetfilename == "" {
				panic("expired hash-ID in texts.json: " + svid)
			}
			var numareas int
			for _, panelareas := range panels {
				numareas += len(panelareas)
			}
			if numareas != 0 {
				importlist[sheetfilename] = sheetfilename + " (" + itoa(numareas) + " text area/s in " + itoa(len(panels)) + " panel/s)"
			}
		}
	}
	s += "</div><h4>Panel editors:</h4><div><ul><li>" + guiHtmlListFrom("importpaneltexts", "(Import panel text areas from another sheet where panel indices match)", true, importlist)
	var importfrom string
	if fv("main_focus_id") == "importpaneltexts" {
		*shouldSaveMeta, importfrom = true, fv("importpaneltexts")
	}
	s += "<li>Default to "
	numtextrects := fv("numtextrects")
	if ui, err := strconv.ParseUint(numtextrects, 10, 64); err != nil {
		numtextrects = itoa(intLim(sv.maxNumTextAreas(), 1, App.Proj.MaxImagePanelTextAreas))
	} else {
		numtextrects = itoa(intLim(int(ui), 1, App.Proj.MaxImagePanelTextAreas))
	}
	s += guiHtmlInput("number", "numtextrects", numtextrects, A{"min": "1", "max": itoa(App.Proj.MaxImagePanelTextAreas), "onchange": "doPostBack('numtextrects')"})
	s += "/" + itoa(App.Proj.MaxImagePanelTextAreas) + " text-rect editor/s</li></ul>"
	if wmax := 480; maxpanelwidth > wmax {
		zoomdiv = float64(wmax) / float64(maxpanelwidth)
		zoom = int(100.0 * zoomdiv)
		zoomdiv = 1.0 / zoomdiv
	}
	savebtnpressed := false
	if rfv := fv("main_focus_id"); rfv != "" && rfv[0] == 'p' && strings.HasSuffix(rfv, "save") {
		*shouldSaveMeta, savebtnpressed, App.Proj.data.Sv.textRects[sv.id] = true, true, nil
	}
	pidx = 0
	sv.data.PanelsTree.iter(func(panel *ImgPanel) {
		rect, pid := panel.Rect, "p"+itoa(pidx)
		w, h := rect.Dx(), rect.Dy()
		cfgdisplay := "none"
		if savebtnpressed {
			App.Proj.data.Sv.textRects[sv.id] = append(App.Proj.data.Sv.textRects[sv.id], []ImgPanelArea{})
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
				trxy := strings.Split(trim(fv(pid+"t"+itoa(i)+"rxy")), ",")
				if len(trxy) < 2 {
					trxy = []string{"", ""}
				}
				trwh := strings.Split(trim(fv(pid+"t"+itoa(i)+"rwh")), ",")
				if len(trwh) < 2 {
					trwh = []string{"", ""}
				}
				if rx0, err := strconv.ParseInt(trxy[0], 0, 64); err == nil {
					if ry0, err := strconv.ParseInt(trxy[1], 0, 64); err == nil {
						if rw, err := strconv.ParseInt(trwh[0], 0, 64); err == nil {
							rx1 := rw + rx0
							if rh, err := strconv.ParseInt(trwh[1], 0, 64); err == nil {
								ry1 := rh + ry0
								area.Rect = image.Rect(int(rx0), int(ry0), int(rx1), int(ry1))
								if rpxy := strings.Split(trim(fv(pid+"t"+itoa(i)+"rpxy")), ","); !area.Rect.Empty() {
									if len(rpxy) < 2 {
										rpxy = []string{"", ""}
									}
									if rpx, rpy := rpxy[0], rpxy[1]; rpx != "" && rpy != "" {
										if rpx, err := strconv.ParseInt(rpx, 0, 64); err == nil {
											if rpy, err := strconv.ParseInt(rpy, 0, 64); err == nil {
												area.PointTo = &image.Point{int(rpx), int(rpy)}
											}
										}
									}
									App.Proj.data.Sv.textRects[sv.id][pidx] = append(App.Proj.data.Sv.textRects[sv.id][pidx], area)
								}
							}
						}
					}
				}
			}
			if panelsareas := App.Proj.data.Sv.textRects[importfrom]; len(panelsareas) > pidx {
				for _, area := range panelsareas[pidx] {
					if !area.Rect.Empty() {
						App.Proj.data.Sv.textRects[sv.fileName][pidx] = append(App.Proj.data.Sv.textRects[sv.fileName][pidx], area)
					}
				}
			}
		}
		langs := []string{}
		for _, lang := range App.Proj.Langs {
			langs = append(langs, lang)
		}
		jsrefr := "refreshPanelRects(" + itoa(pidx) + ", " + itoa(panel.Rect.Min.X) + ", " + itoa(panel.Rect.Min.Y) + ", " + itoa(panel.Rect.Dx()) + ", " + itoa(panel.Rect.Dy()) + ", [\"" + strings.Join(langs, "\", \"") + "\"], " + ftoa(sv.data.PxCm, 8) + ", '" + App.Proj.Gen.PanelSvgText.ClsBoxPoly + "', " + ftoa(App.Proj.Gen.PanelSvgText.BoxPolyStrokeWidthCm, 8) + ");"
		btnhtml := guiHtmlButton(pid+"save", "Save changes (all panels)", A{"onclick": "doPostBack(\"" + pid + "save\")"})

		numpanelareas := len(sv.panelAreas(pidx))
		s += "<hr/><h4 id='pa" + sv.id + itoa(pidx) + "'><u>Panel #" + itoa(pidx+1) + "</u>: " + itoa(numpanelareas) + " text" + strIf(numpanelareas == 1, "", "s")
		for i, lang := range App.Proj.Langs {
			s += "&nbsp;&nbsp;<a href='javascript:refreshAllPanelRects(" + itoa(numpanels) + "," + itoa(i) + ",\"" + lang + "\");'><b>" + lang + "</b></a>"
		}
		s += "</h4><div>Panel coords: " + rect.String() + "</div>"

		s += "<table><tr><td>"
		s += "<div class='panel' style='zoom: " + itoa(zoom) + "%;' onclick='onPanelClick(\"" + pid + "\")'>"
		style := `width: ` + itoa(w) + `px; height: ` + itoa(h) + `px;`
		s += "<div style='position:relative; " + style + "' onauxclick='onPanelAuxClick(event, " + itoa(pidx) + ", " + itoa(panel.Rect.Min.X) + ", " + itoa(panel.Rect.Min.Y) + ", [\"" + strings.Join(langs, "\", \"") + "\"], " + ftoa(zoomdiv, 8) + ")' onmousemove='this.title=parseInt(" + itoa(panel.Rect.Min.X) + "+event.offsetX*" + ftoa(zoomdiv, 8) + ")+\",\"+parseInt(" + itoa(panel.Rect.Min.Y) + "+event.offsetY*" + ftoa(zoomdiv, 8) + ")'>"
		style += `background-image: url("/` + bwsrc + `");`
		style += `background-size: ` + itoa(sv.data.PanelsTree.Rect.Dx()) + `px ` + itoa(sv.data.PanelsTree.Rect.Dy()) + `px;`
		style += `background-position: -` + itoa(rect.Min.X) + `px -` + itoa(rect.Min.Y) + `px;`
		s += "<div class='panelpic' style='" + style + "'></div><span id='" + pid + "rects'></span>"
		s += "</div></div></td><td>"

		s += "<div class='panelcfg' id='" + pid + "cfg' style='text-align: center;display:" + cfgdisplay + ";'>"
		s += "<div>" + btnhtml + "</div>"
		panelareas := sv.panelAreas(pidx)
		for i, ntr := 0, atoi(numtextrects, 1, App.Proj.MaxImagePanelTextAreas); i < App.Proj.MaxImagePanelTextAreas; i++ {
			area, styledisplay := ImgPanelArea{Data: A{}}, "none"
			if i < ntr {
				styledisplay = "inline"
			}
			if len(panelareas) > i {
				area = panelareas[i]
				styledisplay = "inline"
			}

			s += "<span style='display: " + styledisplay + "'><hr/>"
			for _, lang := range App.Proj.Langs {
				s += "<div>" + guiHtmlInput("textarea", pid+"t"+itoa(i)+lang, area.Data[lang], A{
					"placeholder": lang,
					"onfocus":     jsrefr, "onblur": jsrefr, "onchange": jsrefr, "onkeydown": jsrefr, "onkeyup": jsrefr, "onkeypress": jsrefr,
					"style": strings.Replace(strings.Join(App.Proj.Gen.PanelSvgText.Css[""], ";"), "letter-spacing:", "xfoo:", -1),
					"class": "panelcfgtext col" + itoa(i%8)}) + "</div>"
			}

			s += "<div style='text-align: center; white-space: nowrap;'>xy"
			s += guiHtmlInput("text", pid+"t"+itoa(i)+"rxy", itoa(area.Rect.Min.X)+","+itoa(area.Rect.Min.Y), A{"onkeydown": "onDualIntTextInputKeyDown(event)", "onchange": jsrefr, "class": "panelcfgrect"})
			s += "wh"
			s += guiHtmlInput("text", pid+"t"+itoa(i)+"rwh", itoa(area.Rect.Dx())+","+itoa(area.Rect.Dy()), A{"onkeydown": "onDualIntTextInputKeyDown(event)", "onchange": jsrefr, "class": "panelcfgrect"})
			s += "p"
			px, py := "", ""
			if area.PointTo != nil {
				px, py = itoa(area.PointTo.X), itoa(area.PointTo.Y)
			}
			s += guiHtmlInput("text", pid+"t"+itoa(i)+"rpxy", px+","+py, A{"onkeydown": "onDualIntTextInputKeyDown(event)", "onchange": jsrefr, "class": "panelcfgrect"})

			s += "</div><div style='text-align: center;'>" + guiHtmlInput("textarea", pid+"t"+itoa(i)+"_transform", area.SvgTextTransformAttr, A{
				"class": "panelcfgtextattr", "title": "translate(x [,y])\tscale(x [,y])\trotate(a [,oX] [,oY])\tmatrix(a,b,c,d,e,f)\tskewX(x)\tskewY(y)",
				"onfocus": jsrefr, "onblur": jsrefr, "onchange": jsrefr, "onkeydown": jsrefr, "onkeyup": jsrefr, "onkeypress": jsrefr,
			}) + guiHtmlInput("textarea", pid+"t"+itoa(i)+"_style", area.SvgTextTspanStyleAttr, A{
				"class": "panelcfgtextattr", "title": "style attr for the SVG <tspan> element",
				"onfocus": jsrefr, "onblur": jsrefr, "onchange": jsrefr, "onkeydown": jsrefr, "onkeyup": jsrefr, "onkeypress": jsrefr,
			}) + "</div></span>"
		}
		s += "<hr/>" + btnhtml
		s += "</div>"
		s += "</td></tr></table>"
		s += "<script language='javascript' type='text/javascript'>" + jsrefr + "</script>"

		pidx++
	})
	return
}
