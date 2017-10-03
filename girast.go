package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/metaleap/go-util-slice"
)

type GIrANamedTypeRefs []*GIrANamedTypeRef

func (me GIrANamedTypeRefs) Eq(cmp GIrANamedTypeRefs) bool {
	if l := len(me); l != len(cmp) {
		return false
	} else {
		for i := 0; i < l; i++ {
			if !me[i].Eq(cmp[i]) {
				return false
			}
		}
	}
	return true
}

type GIrANamedTypeRef struct {
	Name string `json:",omitempty"`

	RefAlias     string                `json:",omitempty"`
	RefUnknown   int                   `json:",omitempty"`
	RefInterface *GIrATypeRefInterface `json:",omitempty"`
	RefFunc      *GIrATypeRefFunc      `json:",omitempty"`
	RefStruct    *GIrATypeRefStruct    `json:",omitempty"`

	EnumConstNames []string          `json:",omitempty"`
	Methods        GIrANamedTypeRefs `json:",omitempty"`

	methodBody []*CoreImpAst
}

func (me *GIrANamedTypeRef) Eq(cmp *GIrANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.Eq(cmp.RefInterface) && me.RefFunc.Eq(cmp.RefFunc) && me.RefStruct.Eq(cmp.RefStruct))
}

func (me *GIrANamedTypeRef) setFrom(tref interface{}) {
	switch tr := tref.(type) {
	case *GIrATypeRefInterface:
		me.RefInterface = tr
	case *GIrATypeRefFunc:
		me.RefFunc = tr
	case *GIrATypeRefStruct:
		me.RefStruct = tr
	case int:
		me.RefUnknown = tr
	case string:
		me.RefAlias = tr
	case nil:
		me.RefAlias = "voidnilbottomnull"
	default:
		println(tref.(float32))
	}
}

type GIrATypeRefInterface struct {
	Embeds  []string          `json:",omitempty"`
	Methods GIrANamedTypeRefs `json:",omitempty"`
}

func (me *GIrATypeRefInterface) Eq(cmp *GIrATypeRefInterface) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Methods.Eq(cmp.Methods))
}

type GIrATypeRefFunc struct {
	Args GIrANamedTypeRefs `json:",omitempty"`
	Rets GIrANamedTypeRefs `json:",omitempty"`
}

func (me *GIrATypeRefFunc) Eq(cmp *GIrATypeRefFunc) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Args.Eq(cmp.Args) && me.Rets.Eq(cmp.Rets))
}

type GIrATypeRefStruct struct {
	Embeds []string          `json:",omitempty"`
	Fields GIrANamedTypeRefs `json:",omitempty"`
}

func (me *GIrATypeRefStruct) Eq(cmp *GIrATypeRefStruct) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Fields.Eq(cmp.Fields))
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

func (me *GonadIrAst) WriteAsGoTo(writer io.Writer) (err error) {
	var buf = &bytes.Buffer{}

	for _, gtd := range me.girM.GoTypeDefs {
		me.emitGoTypeDecl(buf, gtd, true)
		if len(gtd.EnumConstNames) > 0 {
			fmt.Fprintf(buf, "type %sKinds int\n", gtd.Name)
			fmt.Fprint(buf, "const (\n")
			for i, enumconstname := range gtd.EnumConstNames {
				fmt.Fprintf(buf, "\t%s", enumconstname)
				if i == 0 {
					fmt.Fprintf(buf, " %sKinds = iota", gtd.Name)
				}
				fmt.Fprint(buf, "\n")
			}
			fmt.Fprint(buf, ")\n")
		}
		fmt.Fprintln(buf)
	}

	fmt.Fprintf(writer, "package %s\n\n", me.mod.pName)
	if len(me.girM.Imports) > 0 {
		fmt.Fprint(writer, "import (\n")
		for _, modimp := range me.girM.Imports {
			if modimp.used {
				fmt.Fprintf(writer, "\t%s %q\n", modimp.N, modimp.P)
			} else {
				fmt.Fprintf(writer, "\t// %s %q\n", modimp.N, modimp.P)
			}
		}
		fmt.Fprint(writer, ")\n\n")
	}
	buf.WriteTo(writer)
	return
}

func (me *GonadIrAst) emitGoTypeDecl(w io.Writer, gtd *GIrANamedTypeRef, toplevel bool) {
	fmtembeds := "%s; "
	if toplevel {
		fmtembeds = "\t%s\n"
		fmt.Fprintf(w, "type %s ", gtd.Name)
	}
	if len(gtd.RefAlias) > 0 {
		fmt.Fprint(w, me.emitGoTypeRef(me.resolveGoTypeRef(gtd.RefAlias)))
	} else if gtd.RefUnknown > 0 {
		fmt.Fprintf(w, "interface{/*%d*/}", gtd.RefUnknown)
	} else if gtd.RefInterface != nil {
		fmt.Fprint(w, "interface {")
		if toplevel {
			fmt.Fprintln(w)
		}
		for _, ifaceembed := range gtd.RefInterface.Embeds {
			fmt.Fprintf(w, fmtembeds, me.emitGoTypeRef(me.resolveGoTypeRef(ifaceembed)))
		}
		fmt.Fprint(w, "}")
	} else if gtd.RefStruct != nil {
		fmt.Fprint(w, "struct {")
		if toplevel {
			fmt.Fprintln(w)
		}
		for _, structembed := range gtd.RefStruct.Embeds {
			fmt.Fprintf(w, fmtembeds, structembed)
		}
		for _, structfield := range gtd.RefStruct.Fields {
			var buf bytes.Buffer
			me.emitGoTypeDecl(&buf, structfield, false)
			fmt.Fprintf(w, fmtembeds, structfield.Name+" "+buf.String())
		}
		fmt.Fprint(w, "}")
	} else if gtd.RefFunc != nil {
		fmt.Fprint(w, "func (")
		for i, l := 0, len(gtd.RefFunc.Args); i < l; i++ {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			if argname := gtd.RefFunc.Args[i].Name; len(argname) > 0 {
				fmt.Fprintf(w, "%s ", argname)
			}
			me.emitGoTypeDecl(w, gtd.RefFunc.Args[i], false)
		}
		fmt.Fprint(w, ") (")
		for i, l := 0, len(gtd.RefFunc.Rets); i < l; i++ {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			if retname := gtd.RefFunc.Rets[i].Name; len(retname) > 0 {
				fmt.Fprintf(w, "%s ", retname)
			}
			me.emitGoTypeDecl(w, gtd.RefFunc.Rets[i], false)
		}
		fmt.Fprint(w, ")")
	}
	if toplevel {
		fmt.Fprintln(w)
	}
}

func (me *GonadIrAst) emitGoTypeRef(pname string, tname string) string {
	if len(pname) == 0 {
		return tname
	}
	return pname + "." + tname
}

func (me *GonadIrAst) resolveGoTypeRef(tref string) (pname string, tname string) {
	i := strings.LastIndex(tref, ".")
	if tname = tref[i+1:]; i > 0 {
		pname = tref[:i]
		if pname == me.mod.qName {
			pname = ""
		} else if pname == "Prim" {
			pname = ""
			switch tname {
			case "String":
				tname = "string"
			case "Boolean":
				tname = "bool"
			case "Number":
				tname = "float64"
			case "Int":
				tname = "int"
			default:
				panic("Unknown Prim type: " + tname)
			}
		} else {
			found, qn, mod := false, pname, FindModuleByQName(pname)
			if mod == nil {
				panic(fmt.Errorf("%s: unknown module qname %s", me.mod.srcFilePath, qn))
			}
			pname = mod.pName
			for _, imp := range me.girM.Imports {
				if imp.Q == qn {
					found, imp.used = true, true
				}
			}
			if !found {
				imp := newModImp(mod)
				imp.used, me.girM.imports, me.girM.Imports = true, append(me.girM.imports, mod), append(me.girM.Imports, imp)
			}
		}
	}
	return
}
