package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	nsPrefixDefaultFfiPkg = "Ps2GoFFI."
)

type GonadIrAst struct {
	GIrABlock `json:",omitempty"`

	mod  *ModuleInfo
	proj *BowerProject
	girM *GonadIrMeta
}

type GIrA interface {
	Base() *gIrABase
	Parent() GIrA
}

type gIrABase struct {
	GIrANamedTypeRef `json:",omitempty"` // don't use all of this, but exprs with names and/or types do as needed

	parent GIrA
}

func (me *gIrABase) Base() *gIrABase {
	return me
}

func (me *gIrABase) Parent() GIrA {
	return me.parent
}

type gIrAConstable interface {
	GIrA
	isConstable() bool
}

type GIrAConst struct {
	gIrABase
	ConstVal GIrA `json:",omitempty"`
}

type GIrAVar struct {
	gIrABase
	VarVal GIrA `json:",omitempty"`
}

type GIrAFunc struct {
	gIrABase
	FuncImpl *GIrABlock `json:",omitempty"`
}

type GIrALitStr struct {
	gIrABase
	LitStr string
}

func (me *GIrALitStr) isConstable() bool {
	return true
}

type GIrALitBool struct {
	gIrABase
	LitBool bool
}

func (_ GIrALitBool) isConstable() bool { return true }

type GIrALitDouble struct {
	gIrABase
	LitDouble float64
}

func (_ GIrALitDouble) isConstable() bool { return true }

type GIrALitInt struct {
	gIrABase
	LitInt int
}

func (_ GIrALitInt) isConstable() bool { return true }

type GIrABlock struct {
	gIrABase
	Body []GIrA `json:",omitempty"`
}

func (me *GIrABlock) Add(asts ...GIrA) {
	for _, a := range asts {
		a.Base().parent = me
	}
	me.Body = append(me.Body, asts...)
}

type GIrAComments struct {
	gIrABase
	Comments     []*CoreImpComment `json:",omitempty"`
	CommentsDecl GIrA              `json:",omitempty"`
}

type GIrAOp1 struct {
	gIrABase
	Op1 string `json:",omitempty"`
	Of  GIrA   `json:",omitempty"`
}

func (me GIrAOp1) isConstable() bool {
	if c, ok := me.Of.(gIrAConstable); ok {
		return c.isConstable()
	}
	return false
}

type GIrAOp2 struct {
	gIrABase
	Left  GIrA   `json:",omitempty"`
	Op2   string `json:",omitempty"`
	Right GIrA   `json:",omitempty"`
}

func (me GIrAOp2) isConstable() bool {
	if c, _ := me.Left.(gIrAConstable); c != nil && c.isConstable() {
		if c, _ := me.Right.(gIrAConstable); c != nil {
			return c.isConstable()
		}
	}
	return false
}

type GIrASet struct {
	gIrABase
	SetLeft GIrA `json:",omitempty"`
	ToRight GIrA `json:",omitempty"`

	isInVarGroup bool
}

type GIrAFor struct {
	gIrABase
	ForDo    *GIrABlock `json:",omitempty"`
	ForCond  GIrA       `json:",omitempty"`
	ForInit  []*GIrASet `json:",omitempty"`
	ForStep  []*GIrASet `json:",omitempty"`
	ForRange *GIrAVar   `json:",omitempty"`
}

type GIrAIf struct {
	gIrABase
	If   GIrA       `json:",omitempty"`
	Then *GIrABlock `json:",omitempty"`
	Else *GIrABlock `json:",omitempty"`
}

type GIrACall struct {
	gIrABase
	Callee   GIrA   `json:",omitempty"`
	CallArgs []GIrA `json:",omitempty"`
}

type GIrALitObj struct {
	gIrABase
	ObjFields []*GIrALitObjField `json:",omitempty"`
}

type GIrALitObjField struct {
	gIrABase
	FieldVal GIrA `json:",omitempty"`
}

type GIrANil struct {
	gIrABase
	Nil interface{} // useless except we want to see it in the gonadast.json
}

type GIrARet struct {
	gIrABase
	RetArg GIrA `json:",omitempty"`
}

type GIrAPanic struct {
	gIrABase
	PanicArg GIrA `json:",omitempty"`
}

type GIrALitArr struct {
	gIrABase
	ArrVals []GIrA `json:",omitempty"`
}

type GIrAIndex struct {
	gIrABase
	IdxLeft  GIrA `json:",omitempty"`
	IdxRight GIrA `json:",omitempty"`
}

type GIrADot struct {
	gIrABase
	DotLeft  GIrA `json:",omitempty"`
	DotRight GIrA `json:",omitempty"`
}

type GIrAIsType struct {
	gIrABase
	ExprToTest GIrA `json:",omitempty"`
	TypeToTest GIrA `json:",omitempty"`
}

type GIrAToType struct {
	gIrABase
	ExprToCast GIrA   `json:",omitempty"`
	TypePkg    string `json:",omitempty"`
	TypeName   string `json:",omitempty"`
}

type GIrAPkgRef struct {
	gIrABase
	PkgName string `json:",omitempty"`
	Symbol  string `json:",omitempty"`
}

func (me *GonadIrAst) FinalizePostPrep() (err error) {
	//	various fix-ups
	me.Walk(func(ast GIrA) GIrA {
		if ast != nil {
			switch a := ast.(type) {
			case *GIrAOp1:
				if a != nil && a.Op1 == "&" {
					if oc, _ := a.Of.(*GIrACall); oc != nil {
						return me.FixupAmpCtor(a, oc)
					}
				}
			}
		}
		return ast
	})

	me.LinkTcInstFuncsToImplStructs()
	me.resolveAllArgTypes()
	return
}

func (me *GonadIrAst) PrepFromCoreImp() (err error) {
	//	transform coreimp.json AST into our own leaner Go-focused AST format
	//	mostly focus on discovering new type-defs, final transforms once all
	//	type-defs in all modules are known happen in FinalizePostPrep
	for _, cia := range me.mod.coreimp.Body {
		me.Add(cia.ciAstToGIrAst())
	}

	me.FixupExportedNames()
	me.AddNewExtraTypes()
	nuglobals := me.AddEnumishAdtGlobals()
	me.MiscPrepFixups(nuglobals)
	return
}

func (me *GonadIrAst) resolveAllArgTypes() {
	//	first pass: walk all literals and propagate to parent expressions

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
			enumtypename := toGIrAEnumTypeName(gtd.NamePs)
			codeEmitTypeAlias(buf, enumtypename, "int")
			codeEmitEnumConsts(buf, gtd.EnumConstNames, enumtypename)
		}
		codeEmitTypeMethods(buf, gtd, me.resolveGoTypeRef)
	}

	toplevelconsts := me.topLevelDefs(func(a GIrA) bool { _, ok := a.(*GIrAConst); return ok })
	toplevelvars := me.topLevelDefs(func(a GIrA) bool { _, ok := a.(*GIrAVar); return ok })

	codeEmitGroupedVals(buf, 0, true, toplevelconsts, me.resolveGoTypeRef)
	codeEmitGroupedVals(buf, 0, false, toplevelvars, me.resolveGoTypeRef)

	toplevelctorfuncs := me.topLevelDefs(func(a GIrA) bool { c, ok := a.(*GIrAVar); return ok && c.WasTypeFunc })
	toplevelfuncs := me.topLevelDefs(func(a GIrA) bool { c, ok := a.(*GIrAFunc); return ok && !c.WasTypeFunc })
	for _, ast := range toplevelctorfuncs {
		codeEmitAst(buf, 0, ast, me.resolveGoTypeRef)
		fmt.Fprint(buf, "\n\n")
	}
	for _, ast := range toplevelfuncs {
		codeEmitAst(buf, 0, ast, me.resolveGoTypeRef)
		fmt.Fprint(buf, "\n\n")
	}

	codeEmitPkgDecl(writer, me.mod.pName)
	sort.Sort(me.girM.Imports)
	codeEmitModImps(writer, me.girM.Imports)
	buf.WriteTo(writer)
	return
}

func (me *GonadIrAst) resolveGoTypeRef(tref string, markused bool) (pname string, tname string) {
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
			qn, foundimport, isffi := pname, false, strings.HasPrefix(pname, nsPrefixDefaultFfiPkg)
			var mod *ModuleInfo
			if isffi {
				pname = dot2underscore.Replace(pname)
			} else {
				if mod = FindModuleByQName(pname); mod == nil {
					panic(fmt.Errorf("%s: unknown module qname %s", me.mod.srcFilePath, qn))
				}
				pname = mod.pName
			}
			for _, imp := range me.girM.Imports {
				if imp.Q == qn {
					if foundimport = true; markused {
						imp.used = true
					}
					break
				}
			}
			if !foundimport {
				var imp *GIrMPkgRef
				if isffi {
					imp = &GIrMPkgRef{P: "github.com/metaleap/gonad/" + dot2slash.Replace(qn), Q: qn, N: pname}
				} else {
					imp = newModImp(mod)
				}
				if me.girM.imports, me.girM.Imports = append(me.girM.imports, mod), append(me.girM.Imports, imp); markused {
					imp.used = true
				}
			}
		}
	}
	return
}
