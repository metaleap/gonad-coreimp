package main

import (
	"encoding/json"
	"io"
)

type GonadIrAst struct {
	modinfo *ModuleInfo
	proj    *BowerProject
}

func (me *GonadIrAst) PopulateFromCoreImp() (err error) {
	if err == nil {
	}
	return
}

func (me *GonadIrAst) WriteAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}

func (me *GonadIrAst) WriteAsGoTo(w io.Writer) (err error) {
	return
}
