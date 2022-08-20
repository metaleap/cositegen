package tview_databoundtree

import "reflect"

type DataNode interface {
	Subs() []any
	String() string
}

var NewDataNode = func(data any) DataNode {
	return ReflectionDataNode{Value: reflect.ValueOf(data)}
}

type ReflectionDataNode struct{ reflect.Value }

func (me ReflectionDataNode) Subs() (ret []any) {
	val := me.Value
start:
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		ret = make([]any, val.Len())
		for i := 0; i < len(ret); i++ {
			ret[i] = val.Index(i).Interface()
		}
	case reflect.Map:
		ret = make([]any, val.Len())
		keys := val.MapKeys()
		for i := 0; i < len(ret); i++ {
			ret[i] = val.MapIndex(keys[i]).Interface()
		}
	case reflect.Struct:
		ret = make([]any, 0, val.NumField())
		for i := 0; i < cap(ret); i++ {
			if fld := val.Field(i); fld.CanInterface() {
				ret = append(ret, fld.Interface())
			}
		}
	case reflect.Interface, reflect.Pointer:
		val = val.Elem()
		goto start
	}
	return
}
