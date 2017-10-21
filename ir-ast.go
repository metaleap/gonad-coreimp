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
	nsPrefixDefaultFfiPkg = "ps2goFFI."
)

/*
Golang intermediate-representation AST:
represents the code in a generated Go package, minus
"IR meta stuff" that is, imports & type declarations
(see ir-meta & ir-typestuff), also struct methods.
This latter 'design accident' should probably be revamped.
*/

type gonadIrAst struct {
	gIrABlock `json:",omitempty"`

	culled struct {
		typeCtorFuncs []*gIrACtor
		tcDictDecls   []gIrA
		tcInstDecls   []gIrA
	}
	mod  *modPkg
	proj *psBowerProject
	girM *gonadIrMeta
}

type gIrA interface {
	Ast() *gonadIrAst
	Base() *gIrABase
	Parent() gIrA
}

type gIrABase struct {
	gIrANamedTypeRef `json:",omitempty"` // don't use all of this, but exprs with names and/or types do as needed
	Comments         []*coreImpComment   `json:",omitempty"`
	parent           gIrA
	ast              *gonadIrAst // usually nil but set in top-level gIrABlock. for the rare occasions a gIrA impl needs this, it uses Ast() which traverses parents to the root then stores in ast --- rather than passing the root to all gIrA constructors etc
}

func (me *gIrABase) Ast() *gonadIrAst {
	if me.ast == nil && me.parent != nil {
		me.ast = me.parent.Ast()
	}
	return me.ast
}

func (me *gIrABase) Base() *gIrABase {
	return me
}

func (me *gIrABase) isParentOp() (isparentop bool) {
	if me.parent != nil {
		switch me.parent.(type) {
		case *gIrAOp1, *gIrAOp2:
			isparentop = true
		}
	}
	return
}

func (me *gIrABase) Parent() gIrA {
	return me.parent
}

type gIrAConstable interface {
	isConstable() bool
}

type gIrAConst struct {
	gIrABase
	ConstVal gIrA `json:",omitempty"`
}

func (me *gIrAConst) isConstable() bool { return true }

type gIrALet struct {
	gIrABase
	LetVal gIrA `json:",omitempty"`
}

func (me *gIrALet) isConstable() bool {
	if c, _ := me.LetVal.(gIrAConstable); c != nil {
		return c.isConstable()
	}
	return false
}

type gIrASym struct {
	gIrABase
	refto gIrA
}

func (me *gIrASym) refTo() gIrA {
	if me.refto == nil {
		me.refto = gIrALookupInAncestorBlocks(me, func(stmt gIrA) (isref bool) {
			switch stmt.(type) {
			case *gIrALet, *gIrAConst, *gIrAFunc:
				isref = (me.NamePs == stmt.Base().NamePs)
			}
			return
		})
	}
	return me.refto
}

func (me *gIrASym) isConstable() bool {
	if c, _ := me.refTo().(gIrAConstable); c != nil {
		return c.isConstable()
	}
	return false
}

type gIrAFunc struct {
	gIrABase
	FuncImpl *gIrABlock `json:",omitempty"`
}

type gIrALitStr struct {
	gIrABase
	LitStr string
}

func (me *gIrALitStr) isConstable() bool { return true }

type gIrALitBool struct {
	gIrABase
	LitBool bool
}

func (_ gIrALitBool) isConstable() bool { return true }

type gIrALitDouble struct {
	gIrABase
	LitDouble float64
}

func (_ gIrALitDouble) isConstable() bool { return true }

type gIrALitInt struct {
	gIrABase
	LitInt int
}

func (_ gIrALitInt) isConstable() bool { return true }

type gIrABlock struct {
	gIrABase

	Body []gIrA `json:",omitempty"`
}

func (me *gIrABlock) Add(asts ...gIrA) {
	for _, a := range asts {
		a.Base().parent = me
	}
	me.Body = append(me.Body, asts...)
}

type gIrAComments struct {
	gIrABase
}

type gIrACtor struct {
	gIrAFunc
}

type gIrAOp1 struct {
	gIrABase
	Op1 string `json:",omitempty"`
	Of  gIrA   `json:",omitempty"`
}

func (me gIrAOp1) isConstable() bool {
	if c, ok := me.Of.(gIrAConstable); ok {
		return c.isConstable()
	}
	return false
}

type gIrAOp2 struct {
	gIrABase
	Left  gIrA   `json:",omitempty"`
	Op2   string `json:",omitempty"`
	Right gIrA   `json:",omitempty"`
}

func (me gIrAOp2) isConstable() bool {
	if cl, _ := me.Left.(gIrAConstable); cl != nil && cl.isConstable() {
		if cr, _ := me.Right.(gIrAConstable); cr != nil && cr.isConstable() {
			return true
		}
	}
	return false
}

type gIrASet struct {
	gIrABase
	SetLeft gIrA `json:",omitempty"`
	ToRight gIrA `json:",omitempty"`

	isInVarGroup bool
}

type gIrAFor struct {
	gIrABase
	ForDo    *gIrABlock `json:",omitempty"`
	ForCond  gIrA       `json:",omitempty"`
	ForInit  []*gIrALet `json:",omitempty"`
	ForStep  []*gIrASet `json:",omitempty"`
	ForRange *gIrALet   `json:",omitempty"`
}

type gIrAIf struct {
	gIrABase
	If   gIrA       `json:",omitempty"`
	Then *gIrABlock `json:",omitempty"`
	Else *gIrABlock `json:",omitempty"`
}

type gIrACall struct {
	gIrABase
	Callee   gIrA   `json:",omitempty"`
	CallArgs []gIrA `json:",omitempty"`
}

type gIrALitObj struct {
	gIrABase
	ObjFields []*gIrALitObjField `json:",omitempty"`
}

type gIrALitObjField struct {
	gIrABase
	FieldVal gIrA `json:",omitempty"`
}

type gIrANil struct {
	gIrABase
	Nil interface{} // useless except we want to see it in the gonadast.json
}

type gIrARet struct {
	gIrABase
	RetArg gIrA `json:",omitempty"`
}

type gIrAPanic struct {
	gIrABase
	PanicArg gIrA `json:",omitempty"`
}

type gIrALitArr struct {
	gIrABase
	ArrVals []gIrA `json:",omitempty"`
}

type gIrAIndex struct {
	gIrABase
	IdxLeft  gIrA `json:",omitempty"`
	IdxRight gIrA `json:",omitempty"`
}

type gIrADot struct {
	gIrABase
	DotLeft  gIrA `json:",omitempty"`
	DotRight gIrA `json:",omitempty"`
}

type gIrAIsType struct {
	gIrABase
	ExprToTest gIrA   `json:",omitempty"`
	TypeToTest string `json:",omitempty"`
}

type gIrAToType struct {
	gIrABase
	ExprToCast gIrA   `json:",omitempty"`
	TypePkg    string `json:",omitempty"`
	TypeName   string `json:",omitempty"`
}

type gIrAPkgSym struct {
	gIrABase
	PkgName string `json:",omitempty"`
	Symbol  string `json:",omitempty"`
}

func (me *gonadIrAst) typeCtorFunc(nameps string) *gIrACtor {
	for _, tcf := range me.culled.typeCtorFuncs {
		if tcf.NamePs == nameps {
			return tcf
		}
	}
	return nil
}

func (me *gonadIrAst) finalizePostPrep() {
	//	various fix-ups
	me.walk(func(ast gIrA) gIrA {
		if ast != nil {
			switch a := ast.(type) {
			case *gIrAOp1:
				if a != nil && a.Op1 == "&" {
					if oc, _ := a.Of.(*gIrACall); oc != nil {
						return me.postFixupAmpCtor(a, oc)
					}
				}
			}
		}
		return ast
	})

	me.postLinkTcInstFuncsToImplStructs()
	me.postMiscFixups()
	me.resolveAllArgTypes()
}

func (me *gonadIrAst) prepFromCoreImp() {
	me.gIrABlock.ast = me
	//	transform coreimp.json AST into our own leaner Go-focused AST format
	//	mostly focus on discovering new type-defs, final transforms once all
	//	type-defs in all modules are known happen in FinalizePostPrep
	for _, cia := range me.mod.coreimp.Body {
		me.prepAddOrCull(cia.ciAstToGIrAst())
	}
	me.prepForeigns()
	me.prepFixupExportedNames()
	me.prepAddNewExtraTypes()
	nuglobals := me.prepAddEnumishAdtGlobals()
	me.prepMiscFixups(nuglobals)
}

func (me *gonadIrAst) resolveAllArgTypes() {
	//	first pass: walk all literals and propagate to parent expressions
}

func (me *gonadIrAst) writeAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}

func (me *gonadIrAst) writeAsGoTo(writer io.Writer) (err error) {
	var buf = &bytes.Buffer{}

	sort.Sort(me.girM.GoTypeDefs)
	for _, gtd := range me.girM.GoTypeDefs {
		codeEmitTypeDecl(buf, gtd, 0, me.resolveGoTypeRefFromPsQName)
		codeEmitStructMethods(buf, gtd, me.resolveGoTypeRefFromPsQName)
	}

	toplevelconsts := me.topLevelDefs(func(a gIrA) bool { ac, _ := a.(*gIrAConst); return ac != nil })
	toplevelvars := me.topLevelDefs(func(a gIrA) bool { al, _ := a.(*gIrALet); return al != nil })
	codeEmitGroupedVals(buf, 0, true, toplevelconsts, me.resolveGoTypeRefFromPsQName)
	codeEmitGroupedVals(buf, 0, false, toplevelvars, me.resolveGoTypeRefFromPsQName)

	toplevelfuncs := me.topLevelDefs(func(a gIrA) bool { af, _ := a.(*gIrAFunc); return af != nil })
	for _, ast := range toplevelfuncs {
		codeEmitAst(buf, 0, ast, me.resolveGoTypeRefFromPsQName)
		fmt.Fprint(buf, "\n\n")
	}

	codeEmitPkgDecl(writer, me.mod.pName)
	sort.Sort(me.girM.Imports)
	codeEmitModImps(writer, me.girM.Imports)
	_, err = buf.WriteTo(writer)
	return
}

func (me *gonadIrAst) resolveGoTypeRefFromPsQName(tref string, markused bool) (pname string, tname string) {
	var mod *modPkg
	wasprim := false
	i := strings.LastIndex(tref, ".")
	if tname = tref[i+1:]; i > 0 {
		pname = tref[:i]
		if pname == me.mod.qName {
			pname = ""
			mod = me.mod
		} else if wasprim = pname == "Prim"; wasprim {
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
			if isffi {
				pname = strReplDot2Underscore.Replace(pname)
			} else {
				if mod = findModuleByQName(pname); mod == nil {
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
				var imp *gIrMPkgRef
				if isffi {
					imp = &gIrMPkgRef{P: "github.com/metaleap/gonad/" + strReplDot2Slash.Replace(qn), Q: qn, N: pname}
				} else {
					imp = newModImp(mod)
				}
				if me.girM.imports, me.girM.Imports = append(me.girM.imports, mod), append(me.girM.Imports, imp); markused {
					imp.used = true
				}
			}
		}
	} else {
		mod = me.mod
	}
	if (!wasprim) && mod != nil {
		if tref := mod.girMeta.goTypeDefByPsName(tname); tref != nil {
			tname = mod.girMeta.goTypeDefByPsName(tname).NameGo
		}
	}
	return
}
