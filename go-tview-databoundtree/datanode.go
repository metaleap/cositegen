package tview_databoundtree

import "reflect"

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
	reflval, is := data.(reflect.Value)
	if !is {
		reflval = reflect.ValueOf(data)
	}
	return &reflectionDataNode{reflVal: reflval, parent: parent.(*reflectionDataNode)}
}

type reflectionDataNode struct {
	reflVal reflect.Value
	parent  *reflectionDataNode
}

func (me *reflectionDataNode) String() string {
	return me.reflVal.String()
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
