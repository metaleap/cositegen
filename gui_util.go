package main

import (
	"fmt"
	"html"
)

var hEsc = html.EscapeString

func guiHtmlImg(uri string, attrs map[string]string) string {
	s := "<img src='" + hEsc(uri) + "' " + guiHtmlAttrs(attrs) + "/>"
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

func guiHtmlButton(id string, text string, attrs map[string]string) string {
	s := "<button name='" + hEsc(id) + "' id='" + hEsc(id) + "' " + guiHtmlAttrs(attrs) + ">" + hEsc(text) + "</button>"
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
