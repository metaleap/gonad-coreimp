package ps2goFFI_Data_Show

import (
	"fmt"
	"reflect"
	"strings"
)

type show func(interface{}) string

var (
	ShowIntImpl    show = ShowImpl
	ShowNumberImpl show = ShowImpl
	ShowCharImpl   show = ShowStringImpl
)

func ShowImpl(v interface{}) string {
	return fmt.Sprintf("%#v", v)
}

func ShowStringImpl(v interface{}) string {
	return fmt.Sprintf("%q", v)
}

func ShowArrayImpl(showItemImpl show) show {
	return func(v interface{}) string {
		switch reflect.TypeOf(v).Kind() {
		case reflect.Slice, reflect.Array:
			rsl := reflect.ValueOf(v)
			rsllen := rsl.Len()
			sl := make([]string, rsllen, rsllen)
			for i := 0; i < rsllen; i++ {
				sl[i] = showItemImpl(rsl.Index(i).Interface())
			}
			return "[" + strings.Join(sl, ",") + "]"
		}
		panic(fmt.Errorf("ShowArrayImpl called with %v --- a %v.", v, reflect.TypeOf(v)))
	}
}
