package main

import (
	"fmt"
	"html"
	"strconv"
)

var hEsc = html.EscapeString

func guiHtmlGrayDistrs(grayDistrs [][3]float64) string {
	sum, s := 0.0, "<div class='graydistrs'>"
	for _, gd := range grayDistrs {
		sum += (100 * gd[2])
		spanstyle, cf, ct := "", itoa(int(gd[0])), itoa(int(gd[1])-1)
		if gd[0] > 150 {
			spanstyle = "color: #000000"
		}
		s += "<div style='background: linear-gradient(to right, rgba(" + cf + "," + cf + "," + cf + ",1.0), rgba(" + ct + "," + ct + "," + ct + ",1.0)); min-width: " + itoa(90/len(grayDistrs)) + "%'><span style='" + spanstyle + "'><nobr>" + cf + "-" + ct + "</nobr><br/><b>" + strconv.FormatFloat(100.0*gd[2], 'f', 2, 64) + "%</b><br/><i>(" + strconv.FormatFloat(sum, 'f', 2, 64) + "%)</i>" + "</span></div>"
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
		onchange = "if((this.selectedIndex==0)||confirm(this.options[this.selectedIndex].innerText + \"\\n\\n--- sure about it?\")){" + onchange + "}else{event.cancelBubble=true;this.selectedIndex=0;return false;}"
	}
	s := "<select onchange='" + onchange + "' name='" + hEsc(name) + "' id='" + hEsc(name) + "'>"
	if noneItemFirst != "" {
		s += "<option value=''>" + noneItemFirst + "</option>"
	}
	if getItem != nil {
		for i := 0; i < numItems || numItems == 0; i++ {
			value, caption, sel := getItem(i)
			if value == "" && caption == "" && numItems == 0 {
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
