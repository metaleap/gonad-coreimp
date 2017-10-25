package ps2goFFI_Data_Show

import (
	"fmt"
)

var ShowImpl = func(i interface{}) string {
	return fmt.Sprintf("%v", i)
}

var ShowIntImpl = ShowImpl

var ShowNumberImpl = ShowImpl

var ShowStringImpl = func(s interface{}) string {
	return fmt.Sprintf("%q", s)
}

var ShowCharImpl = ShowStringImpl
