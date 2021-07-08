package main

import (
	"fmt"
	"html"
	"strings"
)

type Indexed interface {
	At(int) fmt.Stringer
	Len() int
}

var hEscs = map[rune]string{
	'ä':  "&auml;",
	'ö':  "&ouml;",
	'ü':  "&uuml;",
	'Ä':  "&Auml;",
	'Ö':  "&Ouml;",
	'Ü':  "&Uuml;",
	'ß':  "&szlig",
	'\n': "&#xA;",
	'’':  "&apos;",
	'…':  "&hellip;",
	'—':  "&mdash;",
	'”':  "&quot;",
	'“':  "&quot;",
}
var hEscRepl *strings.Replacer

func init() {
	repl := make([]string, 0, len(hEscs)*2)
	for k, v := range hEscs {
		repl = append(repl, string([]rune{k}), v)
	}
	hEscRepl = strings.NewReplacer(repl...)
}

func hEsc(s string) string {
	s = html.EscapeString(s)
	for i, r := range s {
		if (r < 32 || r > 127) && hEscs[r] == "" {
			if str := s[i:]; r > 127 {
				if len(str) > 8 {
					str = str[:8]
				}
				println("!!!!!!!!!!!!!NEEDHESC!!!!!!!!!!!!!" + str)
			}
		}
	}
	return hEscRepl.Replace(s)
}

func guiHtmlGrayDistrs(grayDistrs [][3]float64) string {
	sum, s := 0.0, "<div class='graydistrs'>"
	for _, gd := range grayDistrs {
		sum += (100 * gd[2])
		spanstyle, cf, ct := "", itoa(int(gd[0])), itoa(int(gd[1])-1)
		if gd[0] > 150 {
			spanstyle = "color: #000000"
		}
		s += "<div style='background: linear-gradient(to right, rgba(" + cf + "," + cf + "," + cf + ",1.0), rgba(" + ct + "," + ct + "," + ct + ",1.0)); min-width: " + itoa(90/len(grayDistrs)) + "%'><span style='" + spanstyle + "'><nobr>" + cf + "-" + ct + "</nobr><br/><b>" + ftoa(100.0*gd[2], 2) + "%</b><br/><i>(" + ftoa(sum, 2) + "%)</i></span></div>"
	}
	return s + "</div>"
}

func guiHtmlImg(uri string, attrs map[string]string) string {
	s := "<img src='" + hEsc(uri) + "' " + guiHtmlAttrs(attrs) + "/>"
	return s
}

func guiHtmlList(name string, noneItemFirst string, prompt bool, numItems int, getItem func(int) (string, string, bool)) string {
	onchange := "doPostBack(\"" + hEsc(name) + "\");"
	if prompt {
		onchange = "if((this.selectedIndex==0)||confirm(this.options[this.selectedIndex].innerText + \"\\n\\n--- sure about it?\")){" + onchange + "}else{event.stopPropagation();this.selectedIndex=0;return false;}"
	}
	s := "<select onchange='" + onchange + "' name='" + hEsc(name) + "' id='" + hEsc(name) + "'>"
	if noneItemFirst != "" {
		s += "<option value=''>" + noneItemFirst + "</option>"
	}
	if getItem != nil {
		for i := 0; i < numItems || numItems < 0; i++ {
			value, caption, sel := getItem(i)
			if value == "" && caption == "" && numItems < 0 {
				break
			}
			if s += "<option value='" + value + "'"; sel {
				s += " selected"
			}
			s += ">" + hEsc(caption) + "</option>"
		}
	}
	s += "</select>"
	return s
}

func guiHtmlListFrom(name string, noneItemFirst string, prompt bool, from map[string]string) string {
	var keys, vals []string
	for k, v := range from {
		keys, vals = append(keys, k), append(vals, v)
	}
	return guiHtmlList(name, noneItemFirst, prompt, len(from), func(i int) (string, string, bool) { return keys[i], vals[i], false })
}

func guiHtmlButton(id string, text string, attrs map[string]string) string {
	s := "<button type='button' name='" + hEsc(id) + "' id='" + hEsc(id) + "' " + guiHtmlAttrs(attrs) + ">" + hEsc(text) + "</button>"
	return s
}

func guiHtmlInput(inputType string, id string, value string, attrs map[string]string) string {
	if attrs == nil {
		attrs = A{"name": id}
	} else if attrs["name"] == "" {
		attrs["name"] = id
	}
	if defval := attrs["value"]; defval != "" {
		if delete(attrs, "value"); value == "" {
			value = defval
		}
	}
	s := "<input id='" + hEsc(id) + "' type='" + hEsc(inputType) + "' value='" + hEsc(value) + "'"
	if inputType == "textarea" {
		s = "<textarea id='" + hEsc(id) + "'"
	}
	s += guiHtmlAttrs(attrs)
	if inputType == "textarea" {
		s += ">\n" + hEsc(value) + "</textarea>"
	} else {
		s += "/>"
	}
	return s
}

func guiHtmlAttrs(attrs map[string]string) string {
	s := ""
	if attrs != nil {
		for k, v := range attrs {
			s += " " + hEsc(k) + "='" + hEsc(v) + "' "
		}
	}
	return s
}

func guiGetFormSel(formVal string, slice Indexed) fmt.Stringer {
	for i, l := 0, slice.Len(); i < l; i++ {
		if item := slice.At(i); item != nil && item.String() == formVal {
			return item
		}
	}
	return nil
}
