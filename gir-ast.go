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
	Export         bool              `json:",omitempty"`

	mCtor bool
	mBody CoreImpAsts
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
		me.RefAlias = fmt.Sprintf("voidnilbottomnull/*%v*/", tref)
	default:
		println(tref.(float32))
	}
}

type GIrATypeRefInterface struct {
	Embeds  []string          `json:",omitempty"`
	Methods GIrANamedTypeRefs `json:",omitempty"`

	xtc *GIrMTypeClass
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
		codeEmitTypeDecl(buf, gtd, 0, me.resolveGoTypeRef)
		if len(gtd.EnumConstNames) > 0 {
			enumtypename := toGIrAEnumTypeName(gtd.Name)
			codeEmitTypeAlias(buf, enumtypename, "int")
			codeEmitEnumConsts(buf, gtd.EnumConstNames, enumtypename)
			codeEmitTypeMethods(buf, gtd, me.resolveGoTypeRef)
		}
	}

	codeEmitPkgDecl(writer, me.mod.pName)
	codeEmitModImps(writer, me.girM.Imports)
	buf.WriteTo(writer)
	return
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
