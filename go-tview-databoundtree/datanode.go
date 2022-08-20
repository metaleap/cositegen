package tview_databoundtree

import (
	"reflect"
	"strconv"
)

type DataNode interface {
	Subs() []any
	String() string
}

var NewDataNode func(data any, parent DataNode) DataNode

func newDataNode(data any, parent DataNode) DataNode {
	if NewDataNode != nil {
		return NewDataNode(data, parent)
	}
	return newReflectionDataNode(data, parent)
}

func newReflectionDataNode(data any, parent DataNode) DataNode {
	var reflnode reflectionDataNode
	if parent != nil {
		reflnode.parent = parent.(*reflectionDataNode)
	}
	if data != nil {
		if reflval, is := data.(reflect.Value); is {
			reflnode.reflVal = reflval
		} else {
			reflnode.reflVal = reflect.ValueOf(data)
		}
	}
	return &reflnode
}

type reflectionDataNode struct {
	reflVal reflect.Value
	parent  *reflectionDataNode
}

func (me *reflectionDataNode) String() string {
	return reflValString(&me.reflVal)
}

func (me *reflectionDataNode) Subs() (ret []any) {
	val := me.reflVal
start:
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		ret = make([]any, val.Len())
		for i := 0; i < len(ret); i++ {
			ret[i] = val.Index(i)
		}
	case reflect.Map:
		ret = make([]any, val.Len())
		keys := val.MapKeys()
		for i := 0; i < len(ret); i++ {
			ret[i] = val.MapIndex(keys[i])
		}
	case reflect.Struct:
		ret = make([]any, 0, val.NumField())
		for i := 0; i < cap(ret); i++ {
			if fld := val.Field(i); fld.CanInterface() {
				ret = append(ret, fld)
			}
		}
	case reflect.Interface, reflect.Pointer:
		val = val.Elem()
		goto start
	}
	return
}

func (me *reflectionDataNode) isStructField() bool {
	return (me.parent != nil) && (me.parent.reflVal.Kind() == reflect.Struct)
}

func (me *reflectionDataNode) parentReflVal() *reflect.Value {
	if me.parent != nil {
		return &me.parent.reflVal
	}
	return nil
}

func reflValString(v *reflect.Value) (ret string) {
	ret = v.String()
	switch {
	case v.CanInt():
		ret = strconv.FormatInt(v.Int(), 10)
	case v.CanUint():
		ret = strconv.FormatUint(v.Uint(), 10)
	case v.CanFloat():
		ret = strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case v.CanComplex():
		ret = strconv.FormatComplex(v.Complex(), 'f', -1, 128)
	}
	switch v.Kind() {
	case reflect.Bool:
		if ret = "[_[]"; v.Bool() {
			ret = "[×[]"
		}
	case reflect.String:
		ret = strconv.Quote(v.String())
	case reflect.Array:
		ret = "[" + strconv.Itoa(v.Len()) + "[]" + reflTypeString(v.Type().Elem())
	case reflect.Slice:
		ret = "[:" + strconv.Itoa(v.Len()) + "[]" + reflTypeString(v.Type().Elem())
	case reflect.Map:
		ret = strconv.Itoa(v.Len()) + "[" + reflTypeString(v.Type().Key()) + "[]" + reflTypeString(v.Type().Elem())
	case reflect.Struct:
		ret = "{"
		for i := 0; i < v.NumField(); i++ {
			fieldval := v.Field(i)
			ret += ", " + v.Type().Field(i).Name + ": " + reflValString(&fieldval)
		}
		if len(ret) > 1 {
			ret = "{" + ret[2:]
		}
		ret += " }"
	}
	return
}

func reflTypeString(t reflect.Type) (ret string) {
	ret = t.String()
	switch t.Kind() {
	case reflect.Struct:
		ret = "{"
		for i := 0; i < t.NumField(); i++ {
			ret += ", " + t.Field(i).Name
		}
		if len(ret) > 1 {
			ret = "{ " + ret[2:]
		}
		ret += " }"
	}
	return
}
