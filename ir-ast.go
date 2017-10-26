package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/metaleap/go-util/str"
)

/*
Golang intermediate-representation AST:
represents the code in a generated Go package, minus
"IR meta stuff" that is, imports & type declarations
(see ir-meta & ir-typestuff), also struct methods.
This latter 'design accident' should probably be revamped.
*/

const (
	nsPrefixDefaultFfiPkg = "ps2goFFI."
)

var (
	exprTypeInt  = &irANamedTypeRef{RefAlias: "Prim.Int"}
	exprTypeNum  = &irANamedTypeRef{RefAlias: "Prim.Number"}
	exprTypeStr  = &irANamedTypeRef{RefAlias: "Prim.String"}
	exprTypeBool = &irANamedTypeRef{RefAlias: "Prim.Boolean"}
)

type irAst struct {
	irABlock

	culled struct {
		typeCtorFuncs []*irACtor
		tcDictDecls   []irA
		tcInstImpls   []*irTcInstImpl
	}
	mod *modPkg
	irM *irMeta
}

type irTcInstImpl struct {
	tci            *irMTypeClassInst
	tciAlias       string
	tciPassThrough bool
	tciProper      struct {
		tc            *irMTypeClass
		tcMod         *modPkg
		tcMemberImpls []irA
	}
}

type irA interface {
	Ast() *irAst
	Base() *irABase
	Eq(irA) bool // mostly not implemented except where we needed it
	ExprType() *irANamedTypeRef
	Parent() irA
}

type irABase struct {
	irANamedTypeRef                   // don't use all of this, but exprs with names and/or types do as needed
	Comments        []*coreImpComment `json:",omitempty"`
	parent          irA
	root            *irAst // usually nil but set in top-level irABlock. for the rare occasions a irA impl needs this, it uses Ast() which traverses parents to the root then stores in ast --- rather than passing the root to all irA constructors etc
	exprType        *irANamedTypeRef
}

func (me *irABase) Ast() *irAst {
	if me.root == nil && me.parent != nil {
		me.root = me.parent.Ast()
	}
	return me.root
}

func (me *irABase) Base() *irABase {
	return me
}

func (_ *irABase) Eq(_ irA) bool {
	return false
}

func (me *irABase) ExprType() *irANamedTypeRef {
	return me.exprType
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

func (me *irABase) srcFilePath() (srcfilepath string) {
	if root := me.Ast(); root != nil {
		srcfilepath = root.mod.srcFilePath
	}
	return
}

type irAConstable interface {
	isConstable() bool
}

type irAConst struct {
	irABase
	ConstVal irA
}

func (me *irAConst) isConstable() bool { return true }

type irALet struct {
	irABase
	LetVal irA
}

func (me *irALet) isConstable() bool {
	if c, _ := me.LetVal.(irAConstable); c != nil {
		return c.isConstable()
	}
	return false
}

func (me *irALet) ExprType() *irANamedTypeRef {
	if me.exprType == nil {
		if me.LetVal != nil {
			me.exprType = me.LetVal.ExprType()
		}
		if (me.exprType == nil || !me.exprType.hasTypeInfo()) && me.hasTypeInfo() {
			me.exprType = &me.irANamedTypeRef
		}
	}
	return me.exprType
}

type irASym struct {
	irABase
	refto irA
	Sym__ interface{} // useless except we want to see it in the gonadast.json
}

func (me *irASym) Eq(sym irA) bool {
	if s, _ := sym.(*irASym); s != nil {
		if me.NameGo != "" && s.NameGo != "" {
			return me.NameGo == s.NameGo
		} else {
			return me.NamePs == s.NamePs
		}
	}
	return false
}

func (me *irASym) isConstable() bool {
	if c, _ := me.refTo().(irAConstable); c != nil {
		return c.isConstable()
	}
	return false
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

type irAFunc struct {
	irABase
	FuncImpl *irABlock
}

type irALitStr struct {
	irABase
	LitStr string
}

func (_ *irALitStr) ExprType() *irANamedTypeRef { return exprTypeStr }
func (me *irALitStr) isConstable() bool         { return true }

type irALitBool struct {
	irABase
	LitBool bool
}

func (_ *irALitBool) ExprType() *irANamedTypeRef { return exprTypeBool }
func (_ irALitBool) isConstable() bool           { return true }

type irALitNum struct {
	irABase
	LitDouble float64
}

func (_ *irALitNum) ExprType() *irANamedTypeRef { return exprTypeNum }
func (_ irALitNum) isConstable() bool           { return true }

type irALitInt struct {
	irABase
	LitInt int
}

func (_ *irALitInt) ExprType() *irANamedTypeRef { return exprTypeInt }
func (_ irALitInt) isConstable() bool           { return true }

type irABlock struct {
	irABase

	Body []irA
}

func (me *irABlock) add(asts ...irA) {
	for _, a := range asts {
		a.Base().parent = me
	}
	me.Body = append(me.Body, asts...)
}

func (me *irABlock) prepend(asts ...irA) {
	for _, a := range asts {
		a.Base().parent = me
	}
	me.Body = append(asts, me.Body...)
}

type irAComments struct {
	irABase
}

type irACtor struct {
	irAFunc
	Ctor__ interface{} // useless except we want to see it in the gonadast.json
}

type irAOp1 struct {
	irABase
	Op1 string
	Of  irA
}

func (me irAOp1) isConstable() bool {
	if c, ok := me.Of.(irAConstable); ok {
		return c.isConstable()
	}
	return false
}

func (me *irAOp1) ExprType() *irANamedTypeRef {
	if me.Op1 == "!" {
		return exprTypeBool
	} else if me.Of != nil {
		return me.Of.ExprType()
	}
	return me.irABase.ExprType()
}

type irAOp2 struct {
	irABase
	Left  irA
	Op2   string
	Right irA
}

func (me *irAOp2) ExprType() *irANamedTypeRef {
	switch me.Op2 {
	case "==", "!=", "<", "<=", ">", ">=", "&&", "||", "&", "|":
		return exprTypeBool
	default:
		if tl, tr := me.Left.ExprType(), me.Right.ExprType(); tl != nil && tl.hasTypeInfo() {
			return tl
		} else if tr != nil && tr.hasTypeInfo() {
			return tr
		}
	}
	return me.irABase.ExprType()
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
	SetLeft irA
	ToRight irA

	isInVarGroup bool
}

type irAFor struct {
	irABase
	ForDo    *irABlock
	ForCond  irA
	ForInit  []*irALet
	ForStep  []*irASet
	ForRange *irALet
}

type irAIf struct {
	irABase
	If   irA
	Then *irABlock
	Else *irABlock
}

func (_ *irAIf) ExprType() *irANamedTypeRef { return exprTypeBool }

func (me *irAIf) doesCondNegate(other *irAIf) bool {
	mop, _ := me.If.(*irAOp1)
	oop, _ := other.If.(*irAOp1)
	if mop != nil && mop.Op1 != "!" {
		mop = nil
	}
	if oop != nil && oop.Op1 != "!" {
		oop = nil
	}
	if mop == nil && oop != nil {
		return me.If.Eq(oop.Of) // always true so far, but coreimp output formats can always change, so we test correctly
	} else if mop != nil && oop == nil {
		return mop.Of.Eq(other.If) // dito
	}
	return false
}

type irACall struct {
	irABase
	Callee   irA
	CallArgs []irA
}

type irALitObj struct {
	irABase
	ObjFields []*irALitObjField
}

type irALitObjField struct {
	irABase
	FieldVal irA
}

type irANil struct {
	irABase
	Nil__ interface{} // useless except we want to see it in the gonadast.json
}

type irARet struct {
	irABase
	RetArg irA
}

func (me *irARet) ExprType() *irANamedTypeRef {
	if me.exprType == nil && me.RetArg != nil {
		me.exprType = me.RetArg.ExprType()
	}
	return me.exprType
}

type irAPanic struct {
	irABase
	PanicArg irA
}

type irALitArr struct {
	irABase
	ArrVals []irA
}

type irAIndex struct {
	irABase
	IdxLeft  irA
	IdxRight irA
}

type irADot struct {
	irABase
	DotLeft  irA
	DotRight irA
}

type irAIsType struct {
	irABase
	ExprToTest irA
	TypeToTest string
}

func (_ *irAIsType) ExprType() *irANamedTypeRef { return exprTypeBool }

type irAToType struct {
	irABase
	ExprToCast irA
	TypePkg    string
	TypeName   string
}

func (me *irAToType) ExprType() *irANamedTypeRef {
	if me.exprType == nil {
		me.exprType = &irANamedTypeRef{RefAlias: ustr.PrefixWithSep(me.TypePkg, ".", me.TypeName)}
	}
	return me.exprType
}

type irAPkgSym struct {
	irABase
	PkgName string
	Symbol  string
}

func (me *irAPkgSym) ExprType() *irANamedTypeRef {
	if me.exprType == nil {
		if mod := findModuleByPName(me.PkgName); mod != nil {
			if ref := mod.irMeta.goValDeclByGoName(me.Symbol); ref != nil {
				me.exprType = &irANamedTypeRef{}
				me.exprType.copyFrom(ref, false, true, false)
			}
		}
		if me.exprType == nil {
			me.exprType = &irANamedTypeRef{RefAlias: ustr.PrefixWithSep(me.PkgName, ".", me.Symbol)}
		}
	}
	return me.exprType
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

	me.postLinkUpTcMemberFuncs()
	me.postLinkUpTcInstDecls()
	me.postMiscFixups()
	me.postEnsureArgTypes()
	me.postEnsureIfaceCasts()
}

func (me *irAst) prepFromCoreImp() {
	me.irABlock.root = me
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
	me.prepFixupNameCasings()
	nuglobals := me.prepAddEnumishAdtGlobals()
	me.prepMiscFixups(nuglobals)
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
		me.codeGenTypeDef(buf, gtd)
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
