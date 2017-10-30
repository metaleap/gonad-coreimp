package 𝙜ˈDataˈShow

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/gonadz/-"
)

type Shows func(𝒈.𝑻) string

var (
	ShowIntImpl    Shows = ShowImpl
	ShowNumberImpl Shows = ShowImpl
	ShowCharImpl   Shows = ShowStringImpl
)

func ShowImpl(v 𝒈.𝑻) string {
	return fmt.Sprintf("%#v", v)
}

func ShowStringImpl(v 𝒈.𝑻) string {
	return fmt.Sprintf("%q", v)
}

func ShowArrayImpl(showItemImpl Shows) Shows {
	return func(v 𝒈.𝑻) string {
		switch reflect.TypeOf(v).Kind() {
		case reflect.Slice, reflect.Array:
			var buf bytes.Buffer
			buf.WriteRune('[')
			rsl := reflect.ValueOf(v)
			isfirst, rsllen := true, rsl.Len()
			for i := 0; i < rsllen; i++ {
				if isfirst {
					isfirst = false
				} else {
					buf.WriteRune(',')
				}
				buf.WriteString(showItemImpl(rsl.Index(i).Interface()))
			}
			buf.WriteRune(']')
			return buf.String()
		}
		panic(fmt.Errorf("called ShowArrayImpl with %v --- a %v", v, reflect.TypeOf(v)))
	}
}
