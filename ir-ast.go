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

type irAst struct {
	irABlock `json:",omitempty"`

	culled struct {
		typeCtorFuncs []*irACtor
		tcDictDecls   []irA
		tcInstDecls   []irA
	}
	mod  *modPkg
	proj *psBowerProject
	irM  *irMeta
}

type irA interface {
	Ast() *irAst
	Base() *irABase
	Parent() irA
}

type irABase struct {
	irANamedTypeRef `json:",omitempty"` // don't use all of this, but exprs with names and/or types do as needed
	Comments        []*coreImpComment   `json:",omitempty"`
	parent          irA
	ast             *irAst // usually nil but set in top-level irABlock. for the rare occasions a irA impl needs this, it uses Ast() which traverses parents to the root then stores in ast --- rather than passing the root to all irA constructors etc
}

func (me *irABase) Ast() *irAst {
	if me.ast == nil && me.parent != nil {
		me.ast = me.parent.Ast()
	}
	return me.ast
}

func (me *irABase) Base() *irABase {
	return me
}

func (me *irABase) isParentOp() (isparentop bool) {
	if me.parent != nil {
		switch me.parent.(type) {
		case *irAOp1, *irAOp2:
			isparentop = true
		}
	}
	return
}

func (me *irABase) Parent() irA {
	return me.parent
}

type irAConstable interface {
	isConstable() bool
}

type irAConst struct {
	irABase
	ConstVal irA `json:",omitempty"`
}

func (me *irAConst) isConstable() bool { return true }

type irALet struct {
	irABase
	LetVal irA `json:",omitempty"`
}

func (me *irALet) isConstable() bool {
	if c, _ := me.LetVal.(irAConstable); c != nil {
		return c.isConstable()
	}
	return false
}

type irASym struct {
	irABase
	refto irA
}

func (me *irASym) refTo() irA {
	if me.refto == nil {
		me.refto = irALookupInAncestorBlocks(me, func(stmt irA) (isref bool) {
			switch stmt.(type) {
			case *irALet, *irAConst, *irAFunc:
				isref = (me.NamePs == stmt.Base().NamePs)
			}
			return
		})
	}
	return me.refto
}

func (me *irASym) isConstable() bool {
	if c, _ := me.refTo().(irAConstable); c != nil {
		return c.isConstable()
	}
	return false
}

type irAFunc struct {
	irABase
	FuncImpl *irABlock `json:",omitempty"`
}

type irALitStr struct {
	irABase
	LitStr string
}

func (me *irALitStr) isConstable() bool { return true }

type irALitBool struct {
	irABase
	LitBool bool
}

func (_ irALitBool) isConstable() bool { return true }

type irALitNum struct {
	irABase
	LitDouble float64
}

func (_ irALitNum) isConstable() bool { return true }

type irALitInt struct {
	irABase
	LitInt int
}

func (_ irALitInt) isConstable() bool { return true }

type irABlock struct {
	irABase

	Body []irA `json:",omitempty"`
}

func (me *irABlock) Add(asts ...irA) {
	for _, a := range asts {
		a.Base().parent = me
	}
	me.Body = append(me.Body, asts...)
}

type irAComments struct {
	irABase
}

type irACtor struct {
	irAFunc
}

type irAOp1 struct {
	irABase
	Op1 string `json:",omitempty"`
	Of  irA    `json:",omitempty"`
}

func (me irAOp1) isConstable() bool {
	if c, ok := me.Of.(irAConstable); ok {
		return c.isConstable()
	}
	return false
}

type irAOp2 struct {
	irABase
	Left  irA    `json:",omitempty"`
	Op2   string `json:",omitempty"`
	Right irA    `json:",omitempty"`
}

func (me irAOp2) isConstable() bool {
	if cl, _ := me.Left.(irAConstable); cl != nil && cl.isConstable() {
		if cr, _ := me.Right.(irAConstable); cr != nil && cr.isConstable() {
			return true
		}
	}
	return false
}

type irASet struct {
	irABase
	SetLeft irA `json:",omitempty"`
	ToRight irA `json:",omitempty"`

	isInVarGroup bool
}

type irAFor struct {
	irABase
	ForDo    *irABlock `json:",omitempty"`
	ForCond  irA       `json:",omitempty"`
	ForInit  []*irALet `json:",omitempty"`
	ForStep  []*irASet `json:",omitempty"`
	ForRange *irALet   `json:",omitempty"`
}

type irAIf struct {
	irABase
	If   irA       `json:",omitempty"`
	Then *irABlock `json:",omitempty"`
	Else *irABlock `json:",omitempty"`
}

type irACall struct {
	irABase
	Callee   irA   `json:",omitempty"`
	CallArgs []irA `json:",omitempty"`
}

type irALitObj struct {
	irABase
	ObjFields []*irALitObjField `json:",omitempty"`
}

type irALitObjField struct {
	irABase
	FieldVal irA `json:",omitempty"`
}

type irANil struct {
	irABase
	Nil interface{} // useless except we want to see it in the gonadast.json
}

type irARet struct {
	irABase
	RetArg irA `json:",omitempty"`
}

type irAPanic struct {
	irABase
	PanicArg irA `json:",omitempty"`
}

type irALitArr struct {
	irABase
	ArrVals []irA `json:",omitempty"`
}

type irAIndex struct {
	irABase
	IdxLeft  irA `json:",omitempty"`
	IdxRight irA `json:",omitempty"`
}

type irADot struct {
	irABase
	DotLeft  irA `json:",omitempty"`
	DotRight irA `json:",omitempty"`
}

type irAIsType struct {
	irABase
	ExprToTest irA    `json:",omitempty"`
	TypeToTest string `json:",omitempty"`
}

type irAToType struct {
	irABase
	ExprToCast irA    `json:",omitempty"`
	TypePkg    string `json:",omitempty"`
	TypeName   string `json:",omitempty"`
}

type irAPkgSym struct {
	irABase
	PkgName string `json:",omitempty"`
	Symbol  string `json:",omitempty"`
}

func (me *irAst) typeCtorFunc(nameps string) *irACtor {
	for _, tcf := range me.culled.typeCtorFuncs {
		if tcf.NamePs == nameps {
			return tcf
		}
	}
	return nil
}

func (me *irAst) finalizePostPrep() {
	//	various fix-ups
	me.walk(func(ast irA) irA {
		if ast != nil {
			switch a := ast.(type) {
			case *irAOp1:
				if a != nil && a.Op1 == "&" {
					if oc, _ := a.Of.(*irACall); oc != nil {
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

func (me *irAst) prepFromCoreImp() {
	me.irABlock.ast = me
	//	transform coreimp.json AST into our own leaner Go-focused AST format
	//	mostly focus on discovering new type-defs, final transforms once all
	//	type-defs in all modules are known happen in FinalizePostPrep
	for _, cia := range me.mod.coreimp.Body {
		me.prepAddOrCull(cia.ciAstToIrAst())
	}
	for i, tcf := range me.culled.typeCtorFuncs {
		if tcfb := tcf.Base(); tcfb != nil {
			if gtd := me.irM.goTypeDefByPsName(tcfb.NamePs); gtd != nil {
				gtd.sortIndex = i
			}
		}
	}
	me.prepForeigns()
	me.prepFixupExportedNames()
	me.prepAddNewExtraTypes()
	nuglobals := me.prepAddEnumishAdtGlobals()
	me.prepMiscFixups(nuglobals)
}

func (me *irAst) resolveAllArgTypes() {
	//	first pass: walk all literals and propagate to parent expressions
}

func (me *irAst) writeAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}

func (me *irAst) writeAsGoTo(writer io.Writer) (err error) {
	var buf = &bytes.Buffer{}

	sort.Sort(me.irM.GoTypeDefs)
	for _, gtd := range me.irM.GoTypeDefs {
		me.codeGenTypeDecl(buf, gtd, 0)
		me.codeGenStructMethods(buf, gtd)
	}

	toplevelconsts := me.topLevelDefs(func(a irA) bool { ac, _ := a.(*irAConst); return ac != nil })
	toplevelvars := me.topLevelDefs(func(a irA) bool { al, _ := a.(*irALet); return al != nil })
	me.codeGenGroupedVals(buf, true, toplevelconsts)
	me.codeGenGroupedVals(buf, false, toplevelvars)

	toplevelfuncs := me.topLevelDefs(func(a irA) bool { af, _ := a.(*irAFunc); return af != nil })
	for _, ast := range toplevelfuncs {
		me.codeGenAst(buf, 0, ast)
		fmt.Fprint(buf, "\n\n")
	}

	if err = me.codeGenPkgDecl(writer); err == nil {
		if err = me.codeGenModImps(writer); err == nil {
			_, err = buf.WriteTo(writer)
		}
	}
	return
}

func (me *irAst) resolveGoTypeRefFromPsQName(tref string) (pname string, tname string) {
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
			for _, imp := range me.irM.Imports {
				if imp.PsModQName == qn {
					foundimport = true
					break
				}
			}
			if !foundimport {
				var imp *irMPkgRef
				if isffi {
					imp = &irMPkgRef{ImpPath: "github.com/metaleap/gonad/" + strReplDot2Slash.Replace(qn), PsModQName: qn, GoName: pname}
				} else {
					imp = newModImp(mod)
				}
				me.irM.imports, me.irM.Imports = append(me.irM.imports, mod), append(me.irM.Imports, imp)
			}
		}
	} else {
		mod = me.mod
	}
	if (!wasprim) && mod != nil {
		if tref := mod.irMeta.goTypeDefByPsName(tname); tref != nil {
			tname = mod.irMeta.goTypeDefByPsName(tname).NameGo
		}
	}
	return
}
