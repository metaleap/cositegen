package main

import (
	"math"
	"strconv"
	"strings"
)

func xmlOuters(s string, start string, end string) (ret []string) {
	for more := true; more; {
		idx0, idx1, idx2 := strings.LastIndex(s, start[:len(start)-1]+" "), strings.LastIndex(s, start), strings.LastIndex(s, end)
		if idx1 < 0 || idx0 > idx1 {
			idx1 = idx0
		}
		if more = (idx2 > idx1 && idx1 > 0); more {
			ret = append([]string{s[:idx2+len(end)][idx1:]}, ret...)
			s = s[:idx1]
		}
	}
	return
}

func xmlAttrs(s string, names ...string) (ret []string) {
	for _, name := range names {
		ret = append(ret, xmlAttr(s, name))
	}
	return
}

func xmlAttr(s string, name string) string {
	needle := ` ` + name + `="`
	if idx := strings.Index(s, needle); idx > 0 {
		s = s[idx+len(needle):]
		if idx = strings.Index(s, `"`); idx > 0 {
			return s[:idx]
		}
	}
	return ""
}

func xmlInner(s string) string {
	s = s[strings.IndexByte(s, '>')+1:]
	return s[:strings.IndexByte(s, '<')]
}

func cmFromSvgStr(attrVal string, normalize bool) float64 {
	if !strings.HasSuffix(attrVal, "cm") {
		panic(attrVal)
	}
	if f, err := strconv.ParseFloat(attrVal[:len(attrVal)-len("cm")], 64); err != nil {
		panic(err)
	} else {
		if normalize {
			f = 0.1 * math.Ceil(f*10.0)
		}
		return f
	}
}

func (me *SizeAndPos) setSizeAndPosFrom(xml string) {
	me.CmW = cmFromSvgStr(xmlAttr(xml, "svg:width"), true)
	me.CmH = cmFromSvgStr(xmlAttr(xml, "svg:height"), true)
	me.CmX = cmFromSvgStr(xmlAttr(xml, "svg:x"), true)
	me.CmY = cmFromSvgStr(xmlAttr(xml, "svg:y"), true)
}

var xmlRepl = strings.NewReplacer("&apos;", "'")

func (me *Object) setParasFrom(xml string) {
	me.Paras = nil
	for _, xmlpara := range xmlOuters(xml, "<text:p>", "</text:p>") {
		var para string
		for _, xmlspan := range xmlOuters(xmlpara, "<text:span>", "</text:span>") {
			para += xmlInner(xmlspan)
		}
		me.Paras = append(me.Paras, xmlRepl.Replace(strings.TrimSpace(para)))
	}
}
