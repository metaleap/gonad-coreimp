package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type GIrATypeRef interface {
}

type GIrATypeDef struct {
	Name string
	Ref  GIrATypeRef
}

type GIrATypeRefNamed struct {
	Name string
}
type GIrATypeRefVoid struct {
}
type GIrATypeRefUnknown struct {
	Num int
}

type GonadIrAst struct {
	mod  *ModuleInfo
	proj *BowerProject
	girM *GonadIrMeta
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
	var buf bytes.Buffer

	fmt.Fprintf(w, "package %s\n\n", me.mod.pName)
	if len(me.girM.Imports) > 0 {
		fmt.Fprint(w, "import (\n")
		for _, modimp := range me.girM.Imports {
			if modimp.used {
				fmt.Fprintf(w, "\t%s %q\n", modimp.N, modimp.P)
			} else {
				fmt.Fprintf(w, "\t// %s %q\n", modimp.N, modimp.P)
			}
		}
		fmt.Fprint(w, ")\n\n")
	}
	buf.WriteTo(w)
	return
}
