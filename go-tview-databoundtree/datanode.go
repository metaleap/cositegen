package tview_databoundtree

import (
	"fmt"
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
	return newReflectionDataNode(data, parent, nil)
}

func newReflectionDataNode(data any, parent DataNode, prefix any) DataNode {
	ret := reflectionDataNode{prefix: prefix}
	if parent != nil {
		ret.parent = parent.(*reflectionDataNode)
	}
	if data != nil {
		if reflval, is := data.(*reflect.Value); is {
			ret.reflVal = reflval
		} else if reflval, is := data.(reflect.Value); is {
			ret.reflVal = &reflval
		} else {
			reflval = reflect.ValueOf(data)
			ret.reflVal = &reflval
		}
	}
	return &ret
}

type reflectionDataNode struct {
	reflVal *reflect.Value
	parent  *reflectionDataNode
	prefix  any
}

func (me *reflectionDataNode) String() (ret string) {
	if ret = reflValString(me.reflVal); me.prefix != nil {
		var prefstr string
		switch it := me.prefix.(type) {
		case *reflect.Value:
			prefstr = reflValString(it)
		case reflect.Value:
			prefstr = reflValString(&it)
		case reflect.Type:
			prefstr = reflTypeString(it)
		case reflect.StructField:
			prefstr = it.Name
		case string:
			prefstr = strconv.Quote(it)
		case fmt.Stringer:
			prefstr = it.String()
		default:
			prefstr = fmt.Sprintf("%v", it)
		}
		ret = prefstr + ": " + ret
	}
	return
}

func (me *reflectionDataNode) Subs() (ret []any) {
	val := me.reflVal
start:
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		ret = make([]any, val.Len())
		for i := 0; i < len(ret); i++ {
			ret[i] = newReflectionDataNode(val.Index(i), me, i)
		}
	case reflect.Map:
		ret = make([]any, val.Len())
		keys := val.MapKeys()
		for i := 0; i < len(ret); i++ {
			ret[i] = newReflectionDataNode(val.MapIndex(keys[i]), me, &keys[i])
		}
	case reflect.Struct:
		ret = make([]any, 0, val.NumField())
		for i := 0; i < cap(ret); i++ {
			if fld := val.Field(i); fld.CanInterface() && reflFieldTypeOk(fld.Type()) {
				ret = append(ret, newReflectionDataNode(fld, me, val.Type().Field(i)))
			}
		}
	case reflect.Interface, reflect.Pointer:
		v := val.Elem()
		val = &v
		goto start
	}
	return
}

func (me *reflectionDataNode) isStructField() bool {
	return (me.parent != nil) && (me.parent.reflVal.Kind() == reflect.Struct)
}

func (me *reflectionDataNode) parentReflVal() *reflect.Value {
	if me.parent != nil {
		return me.parent.reflVal
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
		if ret = string(rune(0x2610)) + " "; v.Bool() {
			ret = string(rune(0x2611)) + " "
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
	case reflect.Pointer:
		elem := v.Elem()
		return reflValString(&elem)
	}
	return
}

func reflTypeString(t reflect.Type) (ret string) {
	ret = t.String()
	switch t.Kind() {
	case reflect.Interface:
		return "?"
	case reflect.Pointer:
		return reflTypeString(t.Elem())
	case reflect.Array:
		return "[" + strconv.Itoa(t.Len()) + "[]" + reflTypeString(t.Elem())
	case reflect.Slice:
		return "[[]" + reflTypeString(t.Elem())
	case reflect.Map:
		return "[" + reflTypeString(t.Key()) + "[]" + reflTypeString(t.Elem())
	case reflect.Struct:
		ret = "{"
		for i := 0; i < t.NumField(); i++ {
			ret += ", " + t.Field(i).Name
		}
		if len(ret) > 1 {
			ret = "{" + ret[2:]
		}
		ret += " }"
	}
	return
}

func reflFieldTypeOk(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func:
		return false
	case reflect.Map:
		return reflFieldTypeOk(t.Key()) && reflFieldTypeOk(t.Elem())
	case reflect.Array, reflect.Pointer, reflect.Slice:
		return reflFieldTypeOk(t.Elem())
	}
	return true
}
